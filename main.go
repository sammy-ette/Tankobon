package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"

	"tankobon/internal/app"
	"tankobon/internal/repository"
	"tankobon/internal/worker"
)

func main() {
	dbPath := os.Getenv("TANKOBON_DB")
	if dbPath == "" {
		dbPath = "./tankobon.db"
	}
	db, err := repository.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	repository.MigrateSeriesDirs(db)

	wkr := worker.New(db)
	wkr.Start(30*time.Second, 5*time.Minute)

	searcher, err := worker.NewSearcher(db)
	if err != nil {
		log.Fatal(err)
	}
	searcher.Start(24 * time.Hour)

	server := fiber.New()
	server.Use(logger.New())

	server.Get("/api/exists", app.Exists(db))
	server.Post("/api/login", app.Login(db))
	server.Post("/api/register", app.Register(db))
	server.Post("/api/refresh", app.RefreshToken(db))
	server.Get("/api/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"ok":        true,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	protected := server.Group("/api", app.Middleware())
	protected.Post("/scan", app.TriggerScan(wkr, searcher))
	protected.Get("/activity", app.GetActivity(db, wkr))
	protected.Get("/status", app.GetStatus(wkr, searcher))
	protected.Get("/imports", app.ListImports(wkr))
	protected.Get("/imports/history", app.GetImportHistory(db))
	protected.Get("/imports/:hash/files", app.GetImportFiles(db))
	protected.Post("/imports/:hash", app.Import(db, wkr))
	protected.Get("/config", app.GetConfig(db))
	protected.Post("/config", app.SaveConfig(db))
	protected.Get("/series", app.ListSeries(db))
	protected.Get("/series/refresh", app.RefreshAllSeriesMetadata(db))
	protected.Get("/series/search", app.SearchSeries)
	protected.Get("/series/:id", app.GetSeries(db))
	protected.Get("/series/:id/refresh", app.RefreshSeriesMetadata(db))
	protected.Get("/series/:id/search", app.TriggerSeriesSearch(db, searcher))
	protected.Post("/series/:id/search/releases", app.FindReleases(db, searcher))
	protected.Get("/series/:id/search/releases", app.GetReleaseSearch(searcher))
	protected.Post("/series/:id/grab", app.GrabRelease(db))
	protected.Post("/series", app.AddSeries(db, searcher))
	protected.Patch("/series/:id", app.UpdateSeries(db))
	protected.Delete("/series/:id", app.DeleteSeries(db))

	server.Get("/tankobon.js", static.New("./dist/tankobon.js"))
	server.Get("/tankobon.css", static.New("./dist/tankobon.css"))
	server.Get("/*", func(c fiber.Ctx) error {
		return c.SendFile("./dist/index.html")
	})

	log.Fatal(server.Listen(":5505"))
}
