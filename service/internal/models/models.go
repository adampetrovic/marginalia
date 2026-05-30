package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// JSONStringArray is a []string that serializes to JSON in the database.
// Works with both SQLite (text) and PostgreSQL (jsonb).
type JSONStringArray []string

func (j JSONStringArray) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONStringArray) Scan(value interface{}) error {
	if value == nil {
		*j = JSONStringArray{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// JSONMap is a map[string]interface{} that serializes to JSON in the database.
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = JSONMap{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// Source represents a configured highlight input source (e.g. readeck, koreader).
// Sources are per-user: each user has their own readeck/koreader source records.
type Source struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	UserID       string     `gorm:"not null;index" json:"user_id"`
	Type         string     `gorm:"not null" json:"type"` // readeck, koreader
	Name         string     `gorm:"not null" json:"name"`
	Config       JSONMap    `gorm:"type:text" json:"-"` // per-source settings (e.g. readeck url/token)
	LastSyncedAt *time.Time `json:"last_synced_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	Documents []Document `gorm:"foreignKey:SourceID" json:"documents,omitempty"`
	SyncLogs  []SyncLog  `gorm:"foreignKey:SourceID" json:"sync_logs,omitempty"`
}

// Document represents a book, article, podcast, or tweet.
type Document struct {
	ID                string          `gorm:"primaryKey" json:"id"`
	UserID            string          `gorm:"not null;index" json:"user_id"`
	SourceID          string          `gorm:"not null;index;uniqueIndex:idx_source_doc" json:"source_id"`
	SourceDocumentID  string          `gorm:"not null;uniqueIndex:idx_source_doc" json:"source_document_id"`
	Type              string          `gorm:"not null;index" json:"type"` // book, article, podcast, tweet
	Title             string          `gorm:"not null" json:"title"`
	Author            string          `json:"author"`
	URL               string          `json:"url"`
	ImageURL          string          `json:"image_url"`
	SourceURL         string          `json:"source_url"`
	Category          string          `json:"category"`
	Tags              JSONStringArray `gorm:"type:text" json:"tags"`
	Metadata          JSONMap         `gorm:"type:text" json:"metadata"`
	Favorite          bool            `gorm:"not null;default:false;index" json:"favorite"`
	LastHighlightedAt *time.Time      `json:"last_highlighted_at"`
	LastSyncedAt      *time.Time      `json:"last_synced_at"`
	CreatedAt         time.Time       `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time       `gorm:"index" json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`

	Source     Source      `gorm:"foreignKey:SourceID" json:"-"`
	Highlights []Highlight `gorm:"foreignKey:DocumentID" json:"highlights,omitempty"`
}

// Highlight represents an individual highlight/annotation within a document.
type Highlight struct {
	ID                string          `gorm:"primaryKey" json:"id"`
	UserID            string          `gorm:"not null;index" json:"user_id"`
	DocumentID        string          `gorm:"not null;index;uniqueIndex:idx_doc_hl" json:"document_id"`
	SourceHighlightID string          `gorm:"not null;uniqueIndex:idx_doc_hl" json:"source_highlight_id"`
	Text              string          `gorm:"not null" json:"text"`
	Note              string          `json:"note"`
	Color             string          `json:"color"`
	Tags              JSONStringArray `gorm:"type:text" json:"tags"`
	Favorite          bool            `gorm:"not null;default:false;index" json:"favorite"`
	UserEdited        bool            `gorm:"not null;default:false" json:"user_edited"`
	Location          string          `json:"location"`
	LocationType      string          `json:"location_type"` // page, order, time_offset
	LocationSortKey   float64         `gorm:"default:0" json:"location_sort_key"`
	Chapter           string          `json:"chapter"`
	PageNumber        *int            `json:"page_number"`
	Percentage        *float64        `json:"percentage"`
	HighlightedAt     *time.Time      `gorm:"index" json:"highlighted_at"`
	SyncedAt          time.Time       `json:"synced_at"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`

	Document    Document     `gorm:"foreignKey:DocumentID" json:"-"`
	ReviewState *ReviewState `gorm:"foreignKey:HighlightID" json:"review_state,omitempty"`
}

// ReviewState tracks spaced-repetition scheduling for a highlight.
type ReviewState struct {
	HighlightID    string     `gorm:"primaryKey" json:"highlight_id"`
	UserID         string     `gorm:"not null;index" json:"user_id"`
	EaseFactor     float64    `gorm:"not null;default:2.5" json:"ease_factor"`
	IntervalDays   int        `gorm:"not null;default:0" json:"interval_days"`
	Repetitions    int        `gorm:"not null;default:0" json:"repetitions"`
	Lapses         int        `gorm:"not null;default:0" json:"lapses"`
	DueAt          *time.Time `gorm:"index" json:"due_at"`
	LastReviewedAt *time.Time `gorm:"index" json:"last_reviewed_at"`
	LastRating     string     `json:"last_rating"`
	Suspended      bool       `gorm:"not null;default:false;index" json:"suspended"`
	Favorite       bool       `gorm:"not null;default:false" json:"favorite"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	Highlight Highlight `gorm:"foreignKey:HighlightID" json:"-"`
}

// Template stores rendering templates for export.
type Template struct {
	ID                string    `gorm:"primaryKey" json:"id"`
	UserID            string    `gorm:"not null;index" json:"user_id"`
	Name              string    `gorm:"not null" json:"name"`
	Type              string    `gorm:"not null" json:"type"` // book, article, podcast, tweet
	PageTemplate      string    `gorm:"not null" json:"page_template"`
	HighlightTemplate string    `gorm:"not null" json:"highlight_template"`
	IsDefault         bool      `gorm:"default:false" json:"is_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SyncLog records the result of a sync operation.
type SyncLog struct {
	ID               uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           string     `gorm:"not null;index" json:"user_id"`
	SourceID         string     `gorm:"not null;index" json:"source_id"`
	Status           string     `gorm:"not null" json:"status"` // started, completed, failed
	DocumentsSynced  int        `gorm:"default:0" json:"documents_synced"`
	HighlightsSynced int        `gorm:"default:0" json:"highlights_synced"`
	Error            string     `json:"error"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`

	Source Source `gorm:"foreignKey:SourceID" json:"-"`
}
