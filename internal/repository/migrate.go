package repository

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

// MigrateSeriesDirs renames series directories from the old "<title>" format
// to the new "<title> [mb-<id>]" format.
func MigrateSeriesDirs(db *gorm.DB) {
	cfg, err := GetConfig(db)
	if err != nil || cfg.LibraryPath == "" {
		return
	}

	series, err := ListSeries(db)
	if err != nil {
		log.Printf("migrate series dirs: list series: %v\n", err)
		return
	}

	for _, s := range series {
		oldPath := filepath.Join(cfg.LibraryPath, s.Title)
		newPath := filepath.Join(cfg.LibraryPath, fmt.Sprintf("%s [mb-%d]", s.Title, s.MangaBakaID))

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(newPath); err == nil {
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("migrate series dirs ERROR: rename %q -> %q: %v\n", oldPath, newPath, err)
		} else {
			log.Printf("migrate series dirs: renamed %q -> %q\n", oldPath, newPath)
		}
	}
}
