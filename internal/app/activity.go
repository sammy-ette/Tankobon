package app

import (
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"tankobon/internal/clients"
	"tankobon/internal/repository"
	"tankobon/internal/worker"
)

func GetStatus(wkr *worker.Worker, searcher *worker.Searcher) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"worker":   wkr.Status(),
			"searcher": searcher.Status(),
		})
	}
}

func TriggerScan(wkr *worker.Worker, searcher *worker.Searcher) fiber.Handler {
	return func(c fiber.Ctx) error {
		wkr.RunImport()
		searcher.Run()
		return c.SendStatus(202)
	}
}

func GetActivity(db *gorm.DB, wkr *worker.Worker) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := repository.GetConfig(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load config"})
		}
		if cfg.QBittorrentURL == "" {
			return c.JSON(fiber.Map{"activity": []interface{}{}})
		}

		allSeries, err := repository.ListSeries(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to list series"})
		}

		qb := clients.NewQBClient(cfg.QBittorrentURL, cfg.QBittorrentUser, cfg.QBittorrentPass)
		items := wkr.Activity(allSeries, qb, cfg.QBittorrentCategory)

		return c.JSON(fiber.Map{"activity": items})
	}
}
