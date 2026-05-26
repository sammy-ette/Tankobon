package repository

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type MangaContent struct {
	Volumes  map[string]struct{} `json:"-"`
	Chapters map[string]struct{} `json:"-"`
}

func (m MangaContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Volumes  []string `json:"volumes"`
		Chapters []string `json:"chapters"`
	}{
		Volumes:  sortedKeys(m.Volumes),
		Chapters: sortedKeys(m.Chapters),
	})
}

func (m *MangaContent) UnmarshalJSON(data []byte) error {
	var wire struct {
		Volumes  json.RawMessage `json:"volumes"`
		Chapters json.RawMessage `json:"chapters"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Volumes = decodeSet(wire.Volumes)
	m.Chapters = decodeSet(wire.Chapters)
	return nil
}

func decodeSet(raw json.RawMessage) map[string]struct{} {
	set := make(map[string]struct{})
	if len(raw) == 0 {
		return set
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, v := range arr {
			set[v] = struct{}{}
		}
		return set
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for k := range obj {
			set[k] = struct{}{}
		}
	}
	return set
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func NewContent() MangaContent {
	return MangaContent{
		Volumes:  make(map[string]struct{}),
		Chapters: make(map[string]struct{}),
	}
}

func (m *MangaContent) MergeFrom(other MangaContent) {
	if m.Volumes == nil {
		m.Volumes = make(map[string]struct{})
	}
	if m.Chapters == nil {
		m.Chapters = make(map[string]struct{})
	}
	maps.Copy(m.Volumes, other.Volumes)
	maps.Copy(m.Chapters, other.Chapters)
}

func (m MangaContent) Describe() string {
	return fmt.Sprintf("volumes=%v chapters=%v", sortedKeys(m.Volumes), sortedKeys(m.Chapters))
}

func (m *MangaContent) Has(other MangaContent) bool {
	for v := range other.Volumes {
		if _, ok := m.Volumes[v]; ok {
			return true
		}
	}
	for c := range other.Chapters {
		if _, ok := m.Chapters[c]; ok {
			return true
		}
	}
	return false
}

func (m MangaContent) SortedVolumes() []string  { return sortedKeys(m.Volumes) }
func (m MangaContent) SortedChapters() []string { return sortedKeys(m.Chapters) }

func (m MangaContent) HasUncovered(owned MangaContent) bool {
	for v := range m.Volumes {
		if _, ok := owned.Volumes[v]; !ok {
			return true
		}
	}
	for c := range m.Chapters {
		if _, ok := owned.Chapters[c]; !ok {
			return true
		}
	}
	return false
}

type Series struct {
	ID                 uint         `json:"id" gorm:"primaryKey"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
	Title              string       `json:"title" gorm:"not null;index"`
	AltTitles          []string     `json:"altTitles" gorm:"serializer:json"`
	Slug               string       `json:"slug" gorm:"not null;uniqueIndex"`
	Source             string       `json:"source" gorm:"not null"`
	MangaBakaID        int          `json:"mangaBakaId" gorm:"index"`
	Status             string       `json:"status"`
	CoverURL           string       `json:"coverUrl"`
	Overview           string       `json:"overview" gorm:"type:text"`
	Year               int          `json:"year"`
	TotalVolumes       int          `json:"totalVolumes"`
	TotalChapters      int          `json:"totalChapters"   gorm:"column:total_chapters"`
	LastCheckedAt      *time.Time   `json:"lastCheckedAt"   gorm:"column:last_checked_at"`
	Monitored          bool         `json:"monitored" gorm:"not null;default:true;index"`
	MonitorChapters    bool         `json:"monitorChapters" gorm:"not null;default:true"`
	Imported           MangaContent `json:"imported" gorm:"serializer:json"`
	UnmonitoredVolumes []string     `json:"unmonitoredVolumes" gorm:"serializer:json"`
	SeenHashes         []string     `json:"seenHashes" gorm:"serializer:json"`
}

func (s Series) MarshalJSON() ([]byte, error) {
	if s.UnmonitoredVolumes == nil {
		s.UnmonitoredVolumes = []string{}
	}
	if s.SeenHashes == nil {
		s.SeenHashes = []string{}
	}
	if s.AltTitles == nil {
		s.AltTitles = []string{}
	}
	// to avoid stack overflow, use an alias type
	type tmpSeries Series
	return json.Marshal(tmpSeries(s))
}

func (s *Series) HasSeenHash(hash string) bool {
	for _, h := range s.SeenHashes {
		if strings.EqualFold(h, hash) {
			return true
		}
	}
	return false
}

func (s *Series) IsComplete() bool {
	if !strings.EqualFold(s.Status, "completed") {
		return false
	}
	if s.TotalVolumes <= 0 && s.TotalChapters <= 0 {
		return false
	}

	ownedVolumes := append(slices.Collect(maps.Keys(s.Imported.Volumes)), s.UnmonitoredVolumes...)
	ownedChapters := slices.Collect(maps.Keys(s.Imported.Chapters))

	volumesComplete := s.TotalVolumes > 0 && len(ownedVolumes) >= s.TotalVolumes
	chaptersComplete := s.TotalChapters > 0 && len(ownedChapters) >= s.TotalChapters

	return volumesComplete || chaptersComplete
}

func (s *Series) IsVolumeUnmonitored(vol string) bool {
	for _, v := range s.UnmonitoredVolumes {
		if v == vol {
			return true
		}
	}
	return false
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func GetSeries(db *gorm.DB, id uint) (*Series, error) {
	var s Series
	err := db.First(&s, id).Error
	return &s, err
}

func ListSeries(db *gorm.DB) ([]Series, error) {
	var series []Series
	err := db.Order("title asc").Find(&series).Error
	return series, err
}

func ListMonitoredSeries(db *gorm.DB) ([]Series, error) {
	var series []Series
	err := db.Where("monitored = ?", true).Order("title asc").Find(&series).Error
	return series, err
}

func CreateSeries(db *gorm.DB, s *Series) error {
	if s.Slug == "" {
		s.Slug = slugify(s.Title)
	}
	return db.Create(s).Error
}

func UpdateSeriesFields(db *gorm.DB, id uint, updates Series, fields ...string) error {
	tx := db.Model(&Series{}).Where("id = ?", id)
	if len(fields) > 0 {
		tx = tx.Select(fields)
	}
	return tx.Updates(updates).Error
}

func DeleteSeries(db *gorm.DB, id uint) error {
	return db.Delete(&Series{}, id).Error
}

func GetOrCreateSeries(db *gorm.DB, title string) (*Series, error) {
	var s Series
	result := db.Where("title = ?", title).FirstOrCreate(&s, Series{
		Title:  title,
		Slug:   slugify(title),
		Source: "prowlarr",
	})
	return &s, result.Error
}

func AppendSeenHash(db *gorm.DB, s *Series, hash string) error {
	s.SeenHashes = append(s.SeenHashes, hash)
	return UpdateSeriesFields(db, s.ID, Series{SeenHashes: s.SeenHashes}, "SeenHashes")
}

func UpdateSeriesReleaseInfo(db *gorm.DB, id uint, totalVolumes, totalChapters int, checkedAt time.Time) error {
	return db.Model(&Series{}).Where("id = ?", id).Updates(map[string]any{
		"total_volumes":   totalVolumes,
		"total_chapters":  totalChapters,
		"last_checked_at": checkedAt,
	}).Error
}
