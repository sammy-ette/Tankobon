package worker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"path/filepath"
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
}

func NewSearcher(db *gorm.DB) (*Searcher, error) {
	return &Searcher{
		db:       db,
		stopChan: make(chan struct{}),
	}, nil
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
	allSeries, err := repository.ListSeries(s.db)
	if err != nil {
		log.Printf("searcher: list series: %v\n", err)
		return
	}
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
		go func() {
			defer wg.Done()

			storedVols := serie.TotalVolumes
			storedChs := serie.TotalChapters
			now := time.Now()

			if err := limiter.Wait(context.Background()); err != nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			info, err := mbClient.GetSeries(ctx, serie.MangaBakaID)
			cancel()
			if err == nil && info.TotalVolumes != storedVols {
				storedVols = info.TotalVolumes
			} else if err != nil {
				log.Printf("searcher: checkReleases: series %d (%q): mangabaka: %v\n", serie.ID, serie.Title, err)
			}

			if err := repository.UpdateSeriesReleaseInfo(s.db, serie.ID, storedVols, storedChs, now); err != nil {
				log.Printf("searcher: checkReleases: update series %d: %v\n", serie.ID, err)
			}
		}()
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
	for _, series := range monitored {
		if series.IsComplete() {
			log.Printf("searcher: series %d (%q): completed and fully owned, skipping search\n", series.ID, series.Title)
			continue
		}
		created, err := s.TriggerSearch(series, cfg)
		if err != nil {
			log.Printf("searcher: series %d (%q): search failed: %v\n", series.ID, series.Title, err)
			continue
		}
		if created > 0 {
			log.Printf("searcher: series %d (%q): queued torrent\n", series.ID, series.Title)
		}
	}
}

func (s *Searcher) TriggerSearch(series repository.Series, cfg *repository.Config) (int, error) {
	if cfg.ProwlarrURL == "" || cfg.QBittorrentURL == "" {
		return 0, fmt.Errorf("configuration incomplete (Prowlarr or qBittorrent not set)")
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
		return 0, err
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
		return 0, err
	}

	if len(results) == 0 {
		// search for alternative titles
		for _, alt := range series.AltTitles {
			results, err = prowlarrClient.Search(alt)
			if err != nil {
				return 0, err
			}
			if len(results) > 0 {
				break
			}
		}
	}

	type candidate struct {
		result      clients.SearchResult
		torrentData []byte
		hash        string
		score       float64
		content     repository.MangaContent
		shape       release.ReleaseShape
	}

	var (
		candidates []candidate
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	for _, result := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if result.DownloadURL == "" || result.Seeders == 0 {
				return
			}

			titleParsed := release.ParseFile(result.Title)
			if effectiveOwned.Has(titleParsed.Content) {
				return
			}
			if len(titleParsed.Content.Volumes) == 0 && len(titleParsed.Content.Chapters) > 0 && len(effectiveOwned.Volumes) > 0 {
				return
			}

			torrentData, err := prowlarrClient.GetTorrentFile(result.DownloadURL)
			if err != nil {
				log.Printf("search: series %d (%q): get torrent file %q: %v\n", series.ID, series.Title, result.Title, err)
				return
			}

			fileList, hash, err := getFilesFromTorrent(torrentData)
			if err != nil {
				log.Printf("search: series %d (%q): parse torrent %q: %v\n", series.ID, series.Title, result.Title, err)
				return
			}

			if _, err := qbClient.GetTorrent(hash); err == nil {
				return
			}

			if series.HasSeenHash(hash) {
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

			if len(archiveNames) == 0 || !hasNew {
				return
			}
			if len(content.Volumes) == 0 && len(content.Chapters) > 0 && len(effectiveOwned.Volumes) > 0 {
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

			mu.Lock()
			candidates = append(candidates, candidate{
				result:      result,
				torrentData: torrentData,
				hash:        hash,
				score:       shapeScore + float64(result.Seeders*2+result.Leechers),
				content:     content,
				shape:       shape,
			})
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(candidates) == 0 {
		log.Printf("search: series %d (%q): no valid candidates\n", series.ID, series.Title)
		return 0, nil
	}

	added := 0
	for len(candidates) > 0 {
		best, bestIdx := candidates[0], 0
		for i, c := range candidates[1:] {
			if c.score > best.score {
				best, bestIdx = c, i+1
			}
		}
		candidates = append(candidates[:bestIdx], candidates[bestIdx+1:]...)

		log.Printf("search: series %d (%q): selected %q hash=%s (shape=%s score=%.0f content=%s)\n",
			series.ID, series.Title, best.result.Title, best.hash, best.shape, best.score, best.content.Describe())

		if _, err := qbClient.AddTorrentFile(best.torrentData, cfg.QBittorrentCategory); err != nil {
			log.Printf("search: series %d (%q): add torrent %s: %v\n", series.ID, series.Title, best.hash, err)
			continue
		}
		added++
		effectiveOwned.MergeFrom(best.content)

		// drop candidates whose content is now fully covered
		remaining := candidates[:0]
		for _, c := range candidates {
			if c.content.HasUncovered(effectiveOwned) {
				remaining = append(remaining, c)
			}
		}
		candidates = remaining
	}

	return added, nil
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
