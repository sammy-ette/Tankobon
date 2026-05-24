package repository

import (
	"sync/atomic"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var cachedConfig atomic.Pointer[Config]

func InvalidateConfigCache() {
	cachedConfig.Store(nil)
}

type Config struct {
	ID                  uint   `json:"-" gorm:"primaryKey"`
	ProwlarrURL         string `json:"prowlarrURL" gorm:"column:prowlarr_url"`
	ProwlarrAPIToken    string `json:"prowlarrAPIToken" gorm:"column:prowlarr_api_token"`
	QBittorrentURL      string `json:"qbittorrentURL" gorm:"column:qbittorrent_url"`
	QBittorrentUser     string `json:"qbittorrentUser" gorm:"column:qbittorrent_user"`
	QBittorrentPass     string `json:"qbittorrentPass" gorm:"column:qbittorrent_pass"`
	QBittorrentCategory string `json:"qbittorrentCategory" gorm:"column:qbittorrent_category"`
	LibraryPath         string `json:"libraryPath" gorm:"column:library_path"`
}

func GetConfig(db *gorm.DB) (*Config, error) {
	if cfg := cachedConfig.Load(); cfg != nil {
		return cfg, nil
	}
	var cfg Config
	err := db.First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		empty := &Config{}
		cachedConfig.Store(empty)
		return empty, nil
	}
	if err != nil {
		return nil, err
	}
	cachedConfig.Store(&cfg)
	return &cfg, nil
}

func SaveConfig(db *gorm.DB, cfg *Config) error {
	cfg.ID = 1
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(cfg).Error; err != nil {
		return err
	}
	InvalidateConfigCache()
	return nil
}
