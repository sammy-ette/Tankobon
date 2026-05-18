package repository

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Exec(`PRAGMA journal_mode = WAL`)
		sqlDB.Exec(`PRAGMA synchronous = NORMAL`)
		sqlDB.Exec(`PRAGMA temp_store = MEMORY`)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Series{}, &User{}, &Config{}, &ImportLog{})
}
