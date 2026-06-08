package worker

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"tankobon/internal/clients"
	"tankobon/internal/release"
	"tankobon/internal/repository"
)

type PendingReason string

const (
	PendingReasonUnmatched   PendingReason = "unmatched"
	PendingReasonNoFiles     PendingReason = "no_files"
	PendingReasonNeedsReview PendingReason = "needs_review"
)

type PendingImport struct {
	Torrent clients.TorrentInfo     `json:"torrent"`
	Reason  PendingReason           `json:"reason"`
	Series  *repository.Series      `json:"series,omitempty"`
	Shape   release.ReleaseShape    `json:"shape,omitempty"`
	Content repository.MangaContent `json:"content,omitempty"`
}

type ActivityItem struct {
	Torrent    clients.TorrentInfo     `json:"torrent"`
	Series     *repository.Series      `json:"series,omitempty"`
	Content    repository.MangaContent `json:"content,omitempty"`
	Processing bool                    `json:"processing,omitempty"`
}

type Worker struct {
	db *gorm.DB

	mu       sync.RWMutex
	pending  map[string]PendingImport
	wake     chan struct{}
	stopChan chan struct{}
}

func New(db *gorm.DB) *Worker {
	return &Worker{
		db:       db,
		pending:  make(map[string]PendingImport),
		wake:     make(chan struct{}, 1),
		stopChan: make(chan struct{}),
	}
}

// Start runs the worker loop in a new goroutine. The worker will run an import cycle
// every fastInterval when active or pending torrents exist, otherwise every slowInterval.
// This makes the worker more responsive to new torrents while avoiding excessive polling when idle.
func (w *Worker) Start(fastInterval, slowInterval time.Duration) {
	go func() {
		log.Println("worker: running initial import cycle...")
		active := w.cycle()
		for {
			interval := slowInterval
			if active || w.hasPending() {
				interval = fastInterval
			}
			select {
			case <-time.After(interval):
			case <-w.wake:
			case <-w.stopChan:
				return
			}
			active = w.cycle()
		}
	}()
	log.Printf("worker: started (fast=%s slow=%s)\n", fastInterval, slowInterval)
}

// RunImport signals the worker to run an import cycle immediately. Returns false
// if the signal could not be sent (i.e. a cycle is already pending or running).
func (w *Worker) RunImport() bool {
	select {
	case w.wake <- struct{}{}:
		return true
	default:
		return false
	}
}

func (w *Worker) hasPending() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.pending) > 0
}

func (w *Worker) Close() { close(w.stopChan) }

// Activity returns torrents currently active or being processed. Torrents
// already imported (in SeenHashes) and those pending manual attention are
// excluded.
func (w *Worker) Activity(allSeries []repository.Series, qb *clients.QBClient, category string) []ActivityItem {
	torrents, err := qb.GetTorrents(category)
	if err != nil {
		return nil
	}

	seenHashes := make(map[string]bool)
	for _, series := range allSeries {
		for _, h := range series.SeenHashes {
			seenHashes[strings.ToLower(h)] = true
		}
	}

	w.mu.RLock()
	pendingHashes := make(map[string]bool)
	for hash := range w.pending {
		pendingHashes[hash] = true
	}
	w.mu.RUnlock()

	items := make([]ActivityItem, 0, len(torrents))
	for _, t := range torrents {
		hash := strings.ToLower(t.Hash)
		if seenHashes[hash] || pendingHashes[hash] {
			continue
		}
		item := ActivityItem{Torrent: t}
		if matched := matchSeries(t.Name, allSeries); matched != nil {
			item.Series = matched
		}
		if isCompleted(t.State) {
			item.Processing = true
		}
		items = append(items, item)
	}
	return items
}

// Pending returns currently pending imports.
func (w *Worker) Pending() []PendingImport {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]PendingImport, 0, len(w.pending))
	for _, p := range w.pending {
		out = append(out, p)
	}
	return out
}

// PendingByHash returns the pending import for a specific hash, if any.
func (w *Worker) PendingByHash(hash string) (PendingImport, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p, ok := w.pending[strings.ToLower(hash)]
	return p, ok
}

// RemovePending removes a hash from the in-memory pending map.
func (w *Worker) RemovePending(hash string) {
	w.mu.Lock()
	delete(w.pending, strings.ToLower(hash))
	w.mu.Unlock()
}

// cycle polls qBittorrent and imports completed torrents. Returns true
// if any torrents were found in the category (active, downloading, or done).
func (w *Worker) cycle() bool {
	cfg, err := repository.GetConfig(w.db)
	if err != nil {
		log.Printf("worker: failed to load config: %v\n", err)
		return false
	}
	if cfg.QBittorrentURL == "" {
		return false
	}

	qb := clients.NewQBClient(cfg.QBittorrentURL, cfg.QBittorrentUser, cfg.QBittorrentPass)

	torrents, err := qb.GetTorrents(cfg.QBittorrentCategory)
	if err != nil {
		log.Printf("worker: get torrents: %v\n", err)
		return false
	}
	if len(torrents) == 0 {
		return false
	}

	allSeries, err := repository.ListSeries(w.db)
	if err != nil {
		log.Printf("worker: list series: %v\n", err)
		return true
	}

	if cfg.LibraryPath != "" {
		for i, series := range allSeries {
			actual, changed := w.ReconcileImported(series, cfg.LibraryPath)
			if !changed {
				continue
			}
			log.Printf("worker: reconcile series %d (%q): imported diverged from disk, updating\n", series.ID, series.Title)
			if err := repository.UpdateSeriesFields(w.db, series.ID, repository.Series{Imported: actual}, "Imported"); err != nil {
				log.Printf("worker: reconcile series %d: %v\n", series.ID, err)
				continue
			}
			allSeries[i].Imported = actual
		}
	}

	seenHashes := make(map[string]bool)
	for _, series := range allSeries {
		for _, h := range series.SeenHashes {
			seenHashes[strings.ToLower(h)] = true
		}
	}

	newPending := make(map[string]PendingImport)

	for _, t := range torrents {
		if !isCompleted(t.State) {
			continue
		}

		hash := strings.ToLower(t.Hash)
		// log.Printf("worker: considering completed torrent name=%q hash=%s state=%s\n", t.Name, hash, t.State)
		matched := matchSeries(t.Name, allSeries)

		if matched == nil {
			if seenHashes[hash] {
				continue
			}
			newPending[hash] = PendingImport{Torrent: t, Reason: PendingReasonUnmatched}
			continue
		}

		if matched.HasSeenHash(hash) {
			continue
		}

		shape, content, hasFiles := ClassifyFromFiles(t.Hash, t.Name, qb)
		if !hasFiles {
			log.Printf("worker: torrent name=%q series=%d hash=%s: no archive files, marking as pending\n", t.Name, matched.ID, hash)
			newPending[hash] = PendingImport{Torrent: t, Reason: PendingReasonNoFiles, Series: matched}
			continue
		}

		log.Printf("worker: classified torrent name=%q series=%d hash=%s shape=%s content=%s\n", t.Name, matched.ID, hash, shape, content.Describe())

		if len(content.Volumes) == 0 && len(content.Chapters) == 0 {
			log.Printf("worker: torrent name=%q series=%d hash=%s: classification produced no content, marking as needing review\n", t.Name, matched.ID, hash)
			newPending[hash] = PendingImport{Torrent: t, Reason: PendingReasonNeedsReview, Series: matched, Shape: shape, Content: content}
			continue
		}

		importedContent, err := w.Import(t, *matched, nil, cfg)
		if err != nil {
			log.Printf("worker: import %q: %v\n", t.Name, err)
			continue
		}

		matched.Imported.MergeFrom(importedContent)
		matched.SeenHashes = append(matched.SeenHashes, hash)
		log.Printf("worker: imported torrent name=%q series=%d hash=%s content=%s seen_hashes=%d\n", t.Name, matched.ID, hash, importedContent.Describe(), len(matched.SeenHashes))
		if err := repository.UpdateSeriesFields(w.db, matched.ID, repository.Series{SeenHashes: matched.SeenHashes}, "SeenHashes"); err != nil {
			log.Printf("worker: update seen hashes for series %d: %v\n", matched.ID, err)
		}
		w.logImport(matched.Title, t.Name, hash, importedContent)
	}

	w.mu.Lock()
	w.pending = newPending
	w.mu.Unlock()

	return true
}

func (w *Worker) logImport(seriesTitle, torrentName, hash string, content repository.MangaContent) {
	if err := repository.LogImport(w.db, &repository.ImportLog{
		SeriesTitle: seriesTitle,
		TorrentName: torrentName,
		Hash:        hash,
		Content:     content,
	}); err != nil {
		log.Printf("worker: log import: %v\n", err)
	}
}

// ClassifyFromFiles queries qBittorrent for the torrent's file list, parses
// archive names, and returns the release shape and parsed content. The bool
// return is false when no archive files are found.
func ClassifyFromFiles(hash, releaseTitle string, qb *clients.QBClient) (release.ReleaseShape, repository.MangaContent, bool) {
	files, err := qb.GetFiles(hash)
	if err != nil {
		log.Printf("worker: classify %s: get files failed: %v, falling back to title\n", hash, err)
		p := release.ParseFile(releaseTitle)
		return release.Classify(releaseTitle, []string{releaseTitle}), p.Content, true
	}

	var archiveNames []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if release.IsArchive(ext) {
			archiveNames = append(archiveNames, filepath.Base(f.Name))
		}
	}

	if len(archiveNames) == 0 {
		log.Printf("worker: classify %s: no archive files in torrent\n", hash)
		return release.ReleaseShapeUnknown, repository.MangaContent{}, false
	}

	log.Printf("worker: classify %s: found %d archive file(s)\n", hash, len(archiveNames))

	shape := release.Classify(releaseTitle, archiveNames)
	content := repository.NewContent()

	for _, name := range archiveNames {
		content.MergeFrom(release.ParseFile(name).Content)
	}

	return shape, content, true
}

// FileMapping describes a single file within a torrent and the volume/chapter
// numbers it represents, as specified by the user during manual import.
type FileMapping struct {
	Path     string   `json:"path"`
	Volumes  []string `json:"volumes"`
	Chapters []string `json:"chapters"`
	Special  bool     `json:"special"`
}

// ContentFromMappings builds a MangaContent from file mappings, excluding specials.
func ContentFromMappings(mappings []FileMapping) repository.MangaContent {
	content := repository.NewContent()
	for _, m := range mappings {
		if m.Special {
			continue
		}
		for _, v := range m.Volumes {
			content.Volumes[release.NormalizeContentNumber(v)] = struct{}{}
		}
		for _, c := range m.Chapters {
			content.Chapters[release.NormalizeContentNumber(c)] = struct{}{}
		}
	}
	return content
}

// Import hardlinks archive files from the completed torrent into the library
// directory and updates Series.Imported. When mappings are provided, only those
// files are imported with explicit volume/chapter assignments; otherwise all
// archive files in the torrent directory are auto-mapped (decimal chapters become specials).
func (w *Worker) Import(torrent clients.TorrentInfo, series repository.Series, mappings []FileMapping, cfg *repository.Config) (repository.MangaContent, error) {
	if cfg.LibraryPath == "" {
		return repository.MangaContent{}, fmt.Errorf("library path not configured")
	}

	sourceDir, err := TorrentSourceDir(torrent)
	if err != nil {
		return repository.MangaContent{}, err
	}

	if err := checkLinkable(sourceDir, cfg.LibraryPath); err != nil {
		return repository.MangaContent{}, err
	}

	seriesDir := filepath.Join(cfg.LibraryPath, fmt.Sprintf("%s [mb-%d]", series.Title, series.MangaBakaID))
	if err := os.MkdirAll(seriesDir, 0755); err != nil {
		return repository.MangaContent{}, fmt.Errorf("create series dir: %w", err)
	}

	if len(mappings) == 0 {
		mappings, err = FilesToMappings(sourceDir, false)
		if err != nil {
			return repository.MangaContent{}, fmt.Errorf("build mappings: %w", err)
		}
		log.Printf("worker: importing %q to %q\n", sourceDir, seriesDir)
	}

	imported, err := importFiles(sourceDir, seriesDir, series.Title, mappings)
	if err != nil {
		return repository.MangaContent{}, fmt.Errorf("import files: %w", err)
	}
	log.Printf("worker: imported %d file(s) for %q\n", imported, series.Title)

	content := ContentFromMappings(mappings)
	series.Imported.MergeFrom(content)
	if err := repository.UpdateSeriesFields(w.db, series.ID, repository.Series{
		Imported: series.Imported,
	}, "Imported"); err != nil {
		return repository.MangaContent{}, fmt.Errorf("update series: %w", err)
	}

	if cfg.KavitaURL != "" && cfg.KavitaAPIKey != "" {
		kavita := clients.NewKavitaClient(nil, cfg.KavitaURL, cfg.KavitaAPIKey)
		if err := kavita.ScanLibrary(context.Background(), cfg.LibraryPath); err != nil {
			return content, fmt.Errorf("trigger library scan: %w", err)
		}
	}

	return content, nil
}

func importFiles(sourceDir, seriesDir, seriesTitle string, mappings []FileMapping) (int, error) {
	specialsDir := filepath.Join(seriesDir, "Specials")
	cleanSourceDir := filepath.Clean(sourceDir)
	count := 0
	for _, m := range mappings {
		srcPath := filepath.Join(sourceDir, m.Path)
		if !strings.HasPrefix(srcPath, cleanSourceDir+string(filepath.Separator)) {
			return count, fmt.Errorf("invalid path %q: must be within source directory", m.Path)
		}
		ext := strings.ToLower(filepath.Ext(m.Path))

		destName := kavitaNameFromParts(seriesTitle, ext, m.Volumes, m.Chapters)
		var destPath string
		if destName == "" {
			if err := os.MkdirAll(specialsDir, 0755); err != nil {
				return count, fmt.Errorf("create specials dir: %w", err)
			}
			destPath = filepath.Join(specialsDir, filepath.Base(m.Path))
		} else if m.Special {
			if err := os.MkdirAll(specialsDir, 0755); err != nil {
				return count, fmt.Errorf("create specials dir: %w", err)
			}
			destPath = filepath.Join(specialsDir, destName)
		} else {
			destPath = filepath.Join(seriesDir, destName)
		}

		log.Printf("worker: import: %s -> %s\n", m.Path, destPath)
		if err := hardlink(srcPath, destPath); err != nil {
			return count, fmt.Errorf("hardlink %s: %w", m.Path, err)
		}
		count++
	}
	return count, nil
}

// ReconcileImported checks the library directory for a series against the imported
// content in the database. If discrepancies are found
// (e.g. files were added/removed outside of the worker), it returns the actual content
// and true. Otherwise returns false.
func (w *Worker) ReconcileImported(series repository.Series, libraryPath string) (repository.MangaContent, bool) {
	actual := repository.NewContent()

	entries, err := os.ReadDir(filepath.Join(libraryPath, fmt.Sprintf("%s [mb-%d]", series.Title, series.MangaBakaID)))
	if err != nil {
		return series.Imported, false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !release.IsArchive(strings.ToLower(filepath.Ext(name))) {
			continue
		}
		actual.MergeFrom(release.ParseFile(name).Content)
	}

	changed := actual.HasUncovered(series.Imported) || series.Imported.HasUncovered(actual)
	return actual, changed
}

var seriesTitleNonWordRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// normalizeSeriesTitle collapses casing/punctuation differences (e.g. a
// trailing "." or "!") so titles that differ only cosmetically still match.
func normalizeSeriesTitle(s string) string {
	s = strings.ToLower(s)
	s = seriesTitleNonWordRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func matchSeries(torrentName string, allSeries []repository.Series) *repository.Series {
	parsed := release.ParseFile(torrentName)
	if parsed.Series == "" {
		return nil
	}
	fmt.Printf("matching torrent name=%q parsed series=%q alt_series=%v\n", torrentName, parsed.Series, parsed.AltSeries)
	candidates := make(map[string]struct{}, len(parsed.AltSeries)+1)
	for _, c := range append([]string{parsed.Series}, parsed.AltSeries...) {
		candidates[normalizeSeriesTitle(c)] = struct{}{}
	}
	for i, s := range allSeries {
		for _, title := range append([]string{s.Title}, s.AltTitles...) {
			if _, ok := candidates[normalizeSeriesTitle(title)]; ok {
				return &allSeries[i]
			}
		}
	}
	return nil
}

func isCompleted(state string) bool {
	switch state {
	case "uploading", "forcedUP", "stalledUP", "queuedUP", "pausedUP":
		return true
	default:
		return false
	}
}

func kavitaNameFromParts(seriesTitle, ext string, volumes, chapters []string) string {
	switch {
	case len(volumes) > 1:
		return fmt.Sprintf("%s v%s-v%s%s", seriesTitle, zeroPad(volumes[0]), zeroPad(volumes[len(volumes)-1]), ext)
	case len(volumes) == 1:
		return fmt.Sprintf("%s v%s%s", seriesTitle, zeroPad(volumes[0]), ext)
	case len(chapters) > 1:
		return fmt.Sprintf("%s c%s-c%s%s", seriesTitle, zeroPad(chapters[0]), zeroPad(chapters[len(chapters)-1]), ext)
	case len(chapters) == 1:
		return fmt.Sprintf("%s c%s%s", seriesTitle, zeroPad(chapters[0]), ext)
	default:
		return ""
	}
}

func FilesToMappings(sourceDir string, everything bool) ([]FileMapping, error) {
	var mappings []FileMapping
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !everything && !release.IsArchive(strings.ToLower(filepath.Ext(path))) {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		p := release.ParseFile(d.Name())
		mappings = append(mappings, FileMapping{
			Path:     rel,
			Volumes:  p.Content.SortedVolumes(),
			Chapters: p.Content.SortedChapters(),
			Special:  p.Special || InSpecialsDir(rel),
		})
		return nil
	})
	return mappings, err
}

func InSpecialsDir(relPath string) bool {
	dir := filepath.Dir(relPath)
	for dir != "." {
		if strings.EqualFold(filepath.Base(dir), "specials") {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

func kavitaName(seriesTitle, filename, ext string) string {
	p := release.ParseFile(filename)
	return kavitaNameFromParts(seriesTitle, ext, p.Content.SortedVolumes(), p.Content.SortedChapters())
}

func zeroPad(n string) string {
	parts := strings.SplitN(n, ".", 2)
	integer := parts[0]
	if len(integer) < 2 {
		integer = "0" + integer
	}
	if len(parts) == 2 {
		return integer + "." + parts[1]
	}
	return integer
}

func TorrentSourceDir(t clients.TorrentInfo) (string, error) {
	dir := t.ContentPath
	if dir == "" {
		dir = filepath.Join(t.SavePath, t.Name)
	}
	stat, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("source path %q not accessible: %w", dir, err)
	}
	if !stat.IsDir() {
		dir = filepath.Dir(dir)
	}
	return dir, nil
}

func hardlink(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove existing destination: %w", err)
	}
	return os.Link(src, dst)
}
