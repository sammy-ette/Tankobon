package app

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"tankobon/internal/clients"
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
		torrent, err := qb.GetTorrent(hash)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("torrent not found: %v", err)})
		}

		sourceDir, err := worker.TorrentSourceDir(*torrent)
		if err != nil {
			return c.Status(422).JSON(fiber.Map{"error": fmt.Sprintf("torrent files not accessible: %v", err)})
		}

		entries, err := worker.FilesToMappings(sourceDir, true)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("could not read files: %v", err)})
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
			SeriesID     uint                 `json:"seriesId"`
			FileMappings []worker.FileMapping `json:"fileMappings"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.SeriesID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "seriesId is required"})
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

		var mappings []worker.FileMapping
		if len(req.FileMappings) > 0 {
			mappings = req.FileMappings
		} else {
			_, _, hasFiles := worker.ClassifyFromFiles(hash, torrent.Name, qb)
			if !hasFiles {
				return c.Status(422).JSON(fiber.Map{"error": "no archive files found in torrent; provide fileMappings"})
			}
		}
		importedContent, err := wkr.Import(*torrent, *s, mappings, cfg)
		if err != nil {
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
			Content:     importedContent,
		})

		return c.JSON(fiber.Map{"ok": true})
	}
}
