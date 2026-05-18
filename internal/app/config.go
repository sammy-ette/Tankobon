package app

import (
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"tankobon/internal/repository"
)

func GetConfig(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		cfg, err := repository.GetConfig(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load config"})
		}
		return c.JSON(cfg)
	}
}

func SaveConfig(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var cfg repository.Config
		if err := c.Bind().JSON(&cfg); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if err := repository.SaveConfig(db, &cfg); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to save config"})
		}
		return c.JSON(&cfg)
	}
}
