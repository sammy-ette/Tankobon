package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"tankobon/internal/clients"
	"tankobon/internal/release"
	"tankobon/internal/repository"
	"tankobon/internal/worker"
)

func ListImports(wkr *worker.Worker) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"imports": wkr.Pending()})
	}
}

func GetImportFiles(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		hash := strings.ToLower(c.Params("hash"))

		cfg, err := repository.GetConfig(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load config"})
		}

		qb := clients.NewQBClient(cfg.QBittorrentURL, cfg.QBittorrentUser, cfg.QBittorrentPass)
		files, err := qb.GetFiles(hash)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("could not get files: %v", err)})
		}

		type fileEntry struct {
			Path     string   `json:"path"`
			Volumes  []string `json:"volumes"`
			Chapters []string `json:"chapters"`
		}

		entries := make([]fileEntry, 0, len(files))
		for _, f := range files {
			if !release.IsArchive(strings.ToLower(filepath.Ext(f.Name))) {
				continue
			}
			p := release.ParseFile(filepath.Base(f.Name))
			entries = append(entries, fileEntry{Path: f.Name, Volumes: p.Content.SortedVolumes(), Chapters: p.Content.SortedChapters()})
		}

		return c.JSON(fiber.Map{"files": entries})
	}
}

func GetImportHistory(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		entries, err := repository.RecentImports(db, 100)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load history"})
		}
		return c.JSON(fiber.Map{"history": entries})
	}
}

func Import(db *gorm.DB, wkr *worker.Worker) fiber.Handler {
	return func(c fiber.Ctx) error {
		hash := strings.ToLower(c.Params("hash"))

		var req struct {
			SeriesID     uint                 `json:"series_id"`
			FileMappings []worker.FileMapping `json:"file_mappings"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.SeriesID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "series_id is required"})
		}

		cfg, err := repository.GetConfig(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load config"})
		}

		qb := clients.NewQBClient(cfg.QBittorrentURL, cfg.QBittorrentUser, cfg.QBittorrentPass)
		torrent, err := qb.GetTorrent(hash)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("torrent not found in qBittorrent: %v", err)})
		}

		s, err := repository.GetSeries(db, req.SeriesID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "series not found"})
		}

		var content repository.MangaContent
		var mappings []worker.FileMapping
		if len(req.FileMappings) > 0 {
			mappings = req.FileMappings
			content = worker.ContentFromMappings(mappings)
		} else {
			var hasFiles bool
			_, content, hasFiles = worker.ClassifyFromFiles(hash, torrent.Name, qb)
			if !hasFiles {
				return c.Status(422).JSON(fiber.Map{"error": "no archive files found in torrent; provide file_mappings"})
			}
		}
		if err := wkr.Import(*torrent, *s, content, mappings, cfg); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("import failed: %v", err)})
		}

		if err := repository.AppendSeenHash(db, s, hash); err != nil {
			fmt.Printf("manual import: update seen hashes: %v\n", err)
		}
		wkr.RemovePending(hash)
		_ = repository.LogImport(db, &repository.ImportLog{
			SeriesTitle: s.Title,
			TorrentName: torrent.Name,
			Hash:        hash,
			Content:     content,
		})

		return c.JSON(fiber.Map{"ok": true})
	}
}
