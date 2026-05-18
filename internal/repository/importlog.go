package repository

import (
	"time"

	"gorm.io/gorm"
)

type ImportLog struct {
	ID          uint         `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time    `json:"createdAt"`
	SeriesTitle string       `json:"seriesTitle"`
	TorrentName string       `json:"torrentName"`
	Hash        string       `json:"hash"`
	Content     MangaContent `json:"content" gorm:"serializer:json"`
}

func LogImport(db *gorm.DB, entry *ImportLog) error {
	return db.Create(entry).Error
}

func RecentImports(db *gorm.DB, n int) ([]ImportLog, error) {
	var entries []ImportLog
	err := db.Order("created_at desc").Limit(n).Find(&entries).Error
	return entries, err
}
