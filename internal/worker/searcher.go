package worker

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"tankobon/internal/clients"
	"tankobon/internal/release"
	"tankobon/internal/repository"
)

type Searcher struct {
	db *gorm.DB

	releaseMu sync.Mutex
	stopChan  chan struct{}

	statusMu sync.RWMutex
	status   string

	searchMu        sync.Mutex
	releaseSearches map[uint]*ReleaseSearch
}

func NewSearcher(db *gorm.DB) (*Searcher, error) {
	return &Searcher{
		db:              db,
		stopChan:        make(chan struct{}),
		releaseSearches: make(map[uint]*ReleaseSearch),
	}, nil
}

func (s *Searcher) setStatus(status string) {
	s.statusMu.Lock()
	s.status = status
	s.statusMu.Unlock()
}

func (s *Searcher) Status() string {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

func (s *Searcher) Close() {
	close(s.stopChan)
}

func (s *Searcher) Start(interval time.Duration) {
	go func() {
		s.Run()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Run()
			case <-s.stopChan:
				return
			}
		}
	}()
	log.Printf("searcher: started! running every %s\n", interval)
}

func (s *Searcher) Run() bool {
	if !s.releaseMu.TryLock() {
		return false
	}
	go func() {
		defer s.releaseMu.Unlock()
		s.cycle()
	}()
	return true
}

func (s *Searcher) cycle() {
	defer s.setStatus("")

	allSeries, err := repository.ListSeries(s.db)
	if err != nil {
		log.Printf("searcher: list series: %v\n", err)
		return
	}
	s.setStatus("Checking series metadata")
	s.checkSeriesUpdate(allSeries)

	cfg, err := repository.GetConfig(s.db)
	if err != nil {
		log.Printf("searcher: failed to load config: %v\n", err)
		return
	}
	s.triggerSearches(cfg)
}

func (s *Searcher) checkSeriesUpdate(allSeries []repository.Series) {
	mbClient := clients.NewMangabaka(nil)
	limiter := rate.NewLimiter(rate.Every(time.Minute/120), 1)

	var wg sync.WaitGroup
	for _, serie := range allSeries {
		wg.Add(1)
		go func(serie repository.Series) {
			defer wg.Done()

			if err := limiter.Wait(context.Background()); err != nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			info, err := mbClient.GetSeries(ctx, serie.MangaBakaID)
			cancel()
			if err != nil {
				log.Printf("searcher: checkSeriesUpdate: series %d (%q): mangabaka: %v\n", serie.ID, serie.Title, err)
				return
			}

			if err := repository.UpdateFromMangaBakaInfo(s.db, &serie, info); err != nil {
				log.Printf("searcher: checkSeriesUpdate: update series %d: %v\n", serie.ID, err)
			}
		}(serie)
	}
	wg.Wait()
}

func (s *Searcher) triggerSearches(cfg *repository.Config) {
	if cfg.ProwlarrURL == "" {
		return
	}

	monitored, err := repository.ListMonitoredSeries(s.db)
	if err != nil {
		log.Printf("searcher: list monitored series: %v\n", err)
		return
	}

	if len(monitored) == 0 {
		return
	}

	log.Printf("searcher: searching %d monitored series\n", len(monitored))
	for i, series := range monitored {
		if series.IsComplete() {
			log.Printf("searcher: series %d (%q): completed and fully owned, skipping search\n", series.ID, series.Title)
			continue
		}
		s.setStatus(fmt.Sprintf("Searching %q (%d/%d)", series.Title, i+1, len(monitored)))
		candidates, err := s.TriggerSearch(series, cfg, true)
		if err != nil {
			log.Printf("searcher: series %d (%q): search failed: %v\n", series.ID, series.Title, err)
			continue
		}
		grabbed := 0
		for _, c := range candidates {
			if c.Grabbed {
				grabbed++
			}
		}
		if grabbed > 0 {
			log.Printf("searcher: series %d (%q): queued torrent\n", series.ID, series.Title)
		}
	}
}

type Candidate struct {
	Result       clients.SearchResult    `json:"result"`
	Hash         string                  `json:"hash,omitempty"`
	Score        float64                 `json:"score"`
	Content      repository.MangaContent `json:"content,omitempty"`
	Shape        release.ReleaseShape    `json:"shape,omitempty"`
	Approved     bool                    `json:"approved"`
	RejectReason string                  `json:"rejectReason,omitempty"`
	Grabbed      bool                    `json:"grabbed,omitempty"`

	torrentData []byte
}

// TriggerSearch searches for releases for the given series and evaluates every
// result, returning a Candidate for each (approved or not, with a reject
// reason when applicable). When autoGrab is true, the best approved candidates
// are also queued in qBittorrent.
func (s *Searcher) TriggerSearch(series repository.Series, cfg *repository.Config, autoGrab bool) ([]Candidate, error) {
	if cfg.ProwlarrURL == "" || cfg.QBittorrentURL == "" {
		return nil, fmt.Errorf("configuration incomplete (Prowlarr or qBittorrent not set)")
	}

	log.Printf("search: series %d (%q): starting search imported=%s unmonitored=%v seen_hashes=%d\n",
		series.ID, series.Title, series.Imported.Describe(), series.UnmonitoredVolumes, len(series.SeenHashes))

	qbClient := clients.NewQBClient(cfg.QBittorrentURL, cfg.QBittorrentUser, cfg.QBittorrentPass)

	effectiveOwned := repository.NewContent()
	effectiveOwned.MergeFrom(series.Imported)
	for _, v := range series.UnmonitoredVolumes {
		effectiveOwned.Volumes[v] = struct{}{}
	}

	activeTorrents, err := qbClient.GetTorrents(cfg.QBittorrentCategory)
	if err != nil {
		return nil, err
	}

	for _, t := range activeTorrents {
		if matchSeries(t.Name, []repository.Series{series}) != nil {
			if _, content, hasFiles := ClassifyFromFiles(t.Hash, t.Name, qbClient); hasFiles {
				log.Printf("search: series %d (%q): folding active torrent name=%q hash=%s content=%s\n", series.ID, series.Title, t.Name, strings.ToLower(t.Hash), content.Describe())
				effectiveOwned.MergeFrom(content)
			}
		}
	}

	prowlarrClient := clients.NewProwlarr(cfg.ProwlarrURL, cfg.ProwlarrAPIToken)
	results, err := prowlarrClient.Search(series.Title)
	if err != nil {
		return nil, err
	}

	for _, alt := range series.AltTitles {
		otherResults, err := prowlarrClient.Search(alt)
		if err != nil {
			return nil, err
		}

		results = append(results, otherResults...)
	}

	results = slices.Compact(results)

	var (
		candidates []Candidate
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	for _, result := range results {
		fmt.Printf("search: series %d (%q): got result title=%q seeders=%d leechers=%d\n",
			series.ID, series.Title, result.Title, result.Seeders, result.Leechers)
		wg.Go(func() {
			c := Candidate{Result: result}
			reject := func(reason string) {
				c.RejectReason = reason
				mu.Lock()
				candidates = append(candidates, c)
				mu.Unlock()
			}

			// we dont care about dead torrents.
			if result.Seeders == 0 {
				return
			}

			if result.DownloadURL == "" {
				reject("no download URL, turn off \"prefer magnet links\" on the prowlarr indexer config")
				return
			}
			if matchSeries(result.Title, []repository.Series{series}) == nil {
				reject("title does not match series")
				return
			}
			if release.IsRaw(result.Title) {
				reject("raw release")
				return
			}

			titleParsed := release.ParseFile(result.Title)
			if effectiveOwned.Has(titleParsed.Content) {
				reject("release covers content already owned")
				return
			}
			if len(titleParsed.Content.Volumes) == 0 && len(titleParsed.Content.Chapters) > 0 && len(effectiveOwned.Volumes) > 0 {
				reject("chapter-only release not wanted (volumes monitored)")
				return
			}

			torrentData, err := prowlarrClient.GetTorrentFile(result.DownloadURL)
			if err != nil {
				log.Printf("search: series %d (%q): get torrent file %q: %v\n", series.ID, series.Title, result.Title, err)
				reject("failed to fetch torrent file")
				return
			}

			fileList, hash, err := getFilesFromTorrent(torrentData)
			if err != nil {
				log.Printf("search: series %d (%q): parse torrent %q: %v\n", series.ID, series.Title, result.Title, err)
				reject("failed to parse torrent file")
				return
			}
			c.Hash = hash

			if _, err := qbClient.GetTorrent(hash); err == nil {
				reject("already downloading")
				return
			}

			if series.HasSeenHash(hash) {
				reject("already processed this torrent")
				return
			}

			var archiveNames []string
			content := repository.NewContent()
			hasNew := false

			for _, path := range fileList {
				ext := filepath.Ext(path)
				if !release.IsArchive(ext) {
					continue
				}
				name := filepath.Base(path)
				archiveNames = append(archiveNames, name)
				p := release.ParseFile(name)

				content.MergeFrom(p.Content)

				checkContent := p.Content
				if !series.MonitorChapters {
					checkContent.Chapters = nil
				}
				if len(checkContent.Volumes) == 0 && len(checkContent.Chapters) == 0 {
					continue
				}
				if len(checkContent.Volumes) == 0 && len(effectiveOwned.Volumes) > 0 {
					continue
				}
				if !effectiveOwned.Has(checkContent) {
					hasNew = true
				}
			}

			if len(archiveNames) == 0 {
				reject("no archive files in torrent")
				return
			}
			if !hasNew {
				reject("release covers content already owned")
				return
			}
			if len(content.Volumes) == 0 && len(content.Chapters) > 0 && len(effectiveOwned.Volumes) > 0 {
				reject("chapter-only release not wanted (volumes monitored)")
				return
			}

			shape := release.Classify(result.Title, archiveNames)
			var shapeScore float64
			switch shape {
			case release.ReleaseShapeVolumeOnly:
				shapeScore = 5.0
			case release.ReleaseShapeChapterOnly:
				shapeScore = 2.5
			case release.ReleaseShapeMixed:
				shapeScore = 1.5
			}

			c.torrentData = torrentData
			c.Score = shapeScore + float64(result.Seeders*2+result.Leechers)
			c.Content = content
			c.Shape = shape
			c.Approved = true

			mu.Lock()
			candidates = append(candidates, c)
			mu.Unlock()
		})
	}
	wg.Wait()

	if len(candidates) == 0 {
		log.Printf("search: series %d (%q): no results from indexers\n", series.ID, series.Title)
		return candidates, nil
	}

	for i, c := range candidates {
		fmt.Printf("candidate %d: approved=%v score=%.0f title=%q hash=%s shape=%s reason=%q content=%s\n",
			i, c.Approved, c.Score, c.Result.Title, c.Hash, c.Shape, c.RejectReason, c.Content.Describe())
	}

	if !autoGrab {
		return candidates, nil
	}

	var approved []*Candidate
	for i := range candidates {
		if candidates[i].Approved {
			approved = append(approved, &candidates[i])
		}
	}

	if len(approved) == 0 {
		log.Printf("search: series %d (%q): no valid candidates\n", series.ID, series.Title)
	}

	for len(approved) > 0 {
		slices.SortFunc(approved, func(a, b *Candidate) int {
			aVols := a.Content.UncoveredVolumeCount(effectiveOwned)
			bVols := b.Content.UncoveredVolumeCount(effectiveOwned)
			if aVols != bVols {
				return cmp.Compare(bVols, aVols)
			}
			aChs := a.Content.UncoveredChapterCount(effectiveOwned)
			bChs := b.Content.UncoveredChapterCount(effectiveOwned)
			if aChs != bChs {
				return cmp.Compare(bChs, aChs)
			}
			return cmp.Compare(b.Score, a.Score)
		})
		best := approved[0]

		log.Printf("search: series %d (%q): selected %q hash=%s (shape=%s score=%.0f content=%s)\n",
			series.ID, series.Title, best.Result.Title, best.Hash, best.Shape, best.Score, best.Content.Describe())

		if _, err := qbClient.AddTorrentFile(best.torrentData, cfg.QBittorrentCategory); err != nil {
			log.Printf("search: series %d (%q): add torrent %s: %v\n", series.ID, series.Title, best.Hash, err)
			approved = approved[1:]
			continue
		}
		best.Grabbed = true
		effectiveOwned.MergeFrom(best.Content)

		// drop candidates whose content is now fully covered
		remaining := approved[:0]
		for _, c := range approved {
			if len(c.Content.Volumes) == 0 && len(c.Content.Chapters) > 0 && len(effectiveOwned.Volumes) > 0 {
				continue
			}
			if c.Content.HasUncovered(effectiveOwned) {
				remaining = append(remaining, c)
			}
		}
		approved = remaining
	}

	return candidates, nil
}

type ReleaseSearchStatus string

const (
	ReleaseSearchRunning ReleaseSearchStatus = "running"
	ReleaseSearchDone    ReleaseSearchStatus = "done"
	ReleaseSearchFailed  ReleaseSearchStatus = "failed"
)

// ReleaseSearch tracks the progress and result of an on-demand release search
// for a series, evaluated (but not auto-grabbed) via TriggerSearch.
type ReleaseSearch struct {
	Status     ReleaseSearchStatus `json:"status"`
	Candidates []Candidate         `json:"candidates,omitempty"`
	Error      string              `json:"error,omitempty"`
	StartedAt  time.Time           `json:"startedAt"`
}

// FindReleases starts (or returns the status of an already-running) release
// search for the given series. The search runs in the background and
// evaluates every result via TriggerSearch with autoGrab disabled, so the
// caller can poll ReleaseSearchStatus for the result without the request
// blocking on potentially-slow indexer/torrent lookups.
func (s *Searcher) FindReleases(series repository.Series, cfg *repository.Config) ReleaseSearch {
	s.searchMu.Lock()
	if existing, ok := s.releaseSearches[series.ID]; ok && existing.Status == ReleaseSearchRunning {
		cur := *existing
		s.searchMu.Unlock()
		return cur
	}

	rs := &ReleaseSearch{Status: ReleaseSearchRunning, StartedAt: time.Now()}
	s.releaseSearches[series.ID] = rs
	cur := *rs
	s.searchMu.Unlock()

	go func() {
		candidates, err := s.TriggerSearch(series, cfg, false)

		s.searchMu.Lock()
		defer s.searchMu.Unlock()
		if err != nil {
			rs.Status = ReleaseSearchFailed
			rs.Error = err.Error()
			return
		}
		rs.Status = ReleaseSearchDone
		rs.Candidates = candidates
	}()

	return cur
}

// ReleaseSearchStatus returns a snapshot of the most recent release search for
// the given series, if one has been started.
func (s *Searcher) ReleaseSearchStatus(seriesID uint) (ReleaseSearch, bool) {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	rs, ok := s.releaseSearches[seriesID]
	if !ok {
		return ReleaseSearch{}, false
	}
	return *rs, true
}

func getFilesFromTorrent(data []byte) ([]string, string, error) {
	mi, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("load torrent: %w", err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, "", fmt.Errorf("unmarshal info: %w", err)
	}

	hash := mi.HashInfoBytes().HexString()

	var files []string
	for _, f := range info.UpvertedFiles() {
		files = append(files, filepath.Join(f.Path...))
	}

	return files, hash, nil
}
