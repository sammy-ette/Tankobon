package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"tankobon/internal/clients"
	"tankobon/internal/repository"
	"tankobon/internal/worker"
)

type updateSeriesRequest struct {
	UnmonitoredVolumes *[]string `json:"unmonitoredVolumes"`
	Monitored          *bool     `json:"monitored"`
	MonitorChapters    *bool     `json:"monitorChapters"`
}

func ListSeries(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		all, err := repository.ListSeries(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to list series"})
		}
		return c.JSON(fiber.Map{"series": all})
	}
}

func SearchSeries(c fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "q is required"})
	}

	client := clients.NewMangabaka(nil)
	results, err := client.Search(c.Context(), query)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "search failed: " + err.Error()})
	}

	return c.JSON(fiber.Map{"results": results})
}

func AddSeries(db *gorm.DB, searcher *worker.Searcher) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			MangaBakaID int `json:"mangaBakaId"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if req.MangaBakaID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "mangaBakaId is required"})
		}

		baka := clients.NewMangabaka(nil)
		info, err := baka.GetSeries(c.Context(), req.MangaBakaID)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "failed to fetch manga info: " + err.Error()})
		}

		srs := &repository.Series{
			Title:         info.Title,
			Source:        "mangabaka",
			MangaBakaID:   req.MangaBakaID,
			Status:        info.Status,
			CoverURL:      info.CoverURL,
			Overview:      info.Overview,
			Year:          info.Year,
			TotalVolumes:  info.TotalVolumes,
			TotalChapters: info.TotalChapters,
			Monitored:     true,
			Imported:      repository.NewContent(),
		}

		if err := repository.CreateSeries(db, srs); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to add series"})
		}

		if srs.Monitored && searcher != nil {
			cfg, err := repository.GetConfig(db)
			if err == nil {
				go func() {
					if _, err := searcher.TriggerSearch(*srs, cfg); err != nil {
						log.Printf("app: initial search for %q: %v\n", srs.Title, err)
					}
				}()
			}
		}

		return c.SendStatus(201)
	}
}

func TriggerSeriesSearch(db *gorm.DB, searcher *worker.Searcher) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid series id"})
		}

		srs, err := repository.GetSeries(db, uint(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "series not found"})
		}

		cfg, err := repository.GetConfig(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to get config"})
		}

		go func() {
			if _, err := searcher.TriggerSearch(*srs, cfg); err != nil {
				log.Printf("app: manual search for series %d (%q): %v\n", srs.ID, srs.Title, err)
			}
		}()

		return c.SendStatus(200)
	}
}

func GetSeries(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid series id"})
		}
		s, err := repository.GetSeries(db, uint(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "series not found"})
		}
		return c.JSON(s)
	}
}

func RefreshSeriesMetadata(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid series id"})
		}

		srs, err := repository.GetSeries(db, uint(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "series not found"})
		}

		client := clients.NewMangabaka(nil)
		info, err := client.GetSeries(c.Context(), srs.MangaBakaID)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "failed to fetch manga info: " + err.Error()})
		}

		if err := repository.UpdateFromMangaBakaInfo(db, srs, info); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to update series: " + err.Error()})
		}

		return c.JSON(srs)
	}
}

func RefreshAllSeriesMetadata(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		all, err := repository.ListSeries(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to list series"})
		}

		go func() {
			for _, srs := range all {
				client := clients.NewMangabaka(nil)
				info, err := client.GetSeries(c.Context(), srs.MangaBakaID)
				if err != nil {
					fmt.Printf("failed to fetch manga info for %s: %s\n", srs.Title, err.Error())
					return
				}

				if err := repository.UpdateFromMangaBakaInfo(db, &srs, info); err != nil {
					fmt.Printf("failed to update series metadata for %s: %s\n", srs.Title, err.Error())
					return
				}
			}
		}()

		return c.SendStatus(200)
	}
}

func UpdateSeries(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid series id"})
		}

		var req updateSeriesRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}

		updates := repository.Series{}
		var fields []string
		if req.UnmonitoredVolumes != nil {
			updates.UnmonitoredVolumes = *req.UnmonitoredVolumes
			fields = append(fields, "UnmonitoredVolumes")
		}
		if req.Monitored != nil {
			updates.Monitored = *req.Monitored
			fields = append(fields, "Monitored")
		}
		if req.MonitorChapters != nil {
			updates.MonitorChapters = *req.MonitorChapters
			fields = append(fields, "MonitorChapters")
		}

		if len(fields) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no valid fields to update"})
		}

		if err := repository.UpdateSeriesFields(db, uint(id), updates, fields...); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to update series"})
		}

		updated, err := repository.GetSeries(db, uint(id))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch updated series"})
		}

		return c.JSON(updated)
	}
}

func DeleteSeries(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid series id"})
		}

		deleteFiles := c.Query("deleteFiles") == "true"

		var seriesDir, libraryPath string
		if deleteFiles {
			s, err := repository.GetSeries(db, uint(id))
			if err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "series not found"})
			}
			seriesDir = fmt.Sprintf("%s [mb-%d]", s.Title, s.MangaBakaID)

			cfg, err := repository.GetConfig(db)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to get config"})
			}
			libraryPath = cfg.LibraryPath
		}

		if err := repository.DeleteSeries(db, uint(id)); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to delete series"})
		}

		if deleteFiles && libraryPath != "" && seriesDir != "" {
			if err := os.RemoveAll(filepath.Join(libraryPath, seriesDir)); err != nil {
				log.Printf("delete series files: %v\n", err)
			}
		}

		return c.JSON(fiber.Map{"ok": true})
	}
}
