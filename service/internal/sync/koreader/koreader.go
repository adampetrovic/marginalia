package koreader

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

const sourceType = "koreader"

// ReadwiseHighlightRequest is the Readwise-compatible payload that KOReader sends.
type ReadwiseHighlightRequest struct {
	Highlights []ReadwiseHighlight `json:"highlights"`
}

// ReadwiseHighlight is a single highlight in Readwise API format.
type ReadwiseHighlight struct {
	Text          string  `json:"text"`
	Title         string  `json:"title"`
	Author        string  `json:"author"`
	SourceType    string  `json:"source_type"`
	Category      string  `json:"category"`
	Note          string  `json:"note"`
	Location      int     `json:"location"`
	LocationType  string  `json:"location_type"`
	HighlightedAt string  `json:"highlighted_at"` // ISO 8601
}

// IngestResult holds the outcome of processing a KOReader push.
type IngestResult struct {
	DocumentsSynced  int
	HighlightsSynced int
}

// Ingest processes a Readwise-format highlight push from KOReader.
func Ingest(db *gorm.DB, sourceID string, req ReadwiseHighlightRequest) (*IngestResult, error) {
	result := &IngestResult{}

	// Group highlights by book (title + author)
	type bookKey struct {
		title  string
		author string
	}
	grouped := make(map[bookKey][]ReadwiseHighlight)
	for _, hl := range req.Highlights {
		key := bookKey{title: hl.Title, author: hl.Author}
		grouped[key] = append(grouped[key], hl)
	}

	for key, highlights := range grouped {
		// Generate a stable document ID from source + title + author
		docSourceID := fmt.Sprintf("%s:%s", key.title, key.author)

		doc := models.Document{
			ID:               fmt.Sprintf("koreader-%s", sanitizeID(docSourceID)),
			SourceID:         sourceID,
			SourceDocumentID: docSourceID,
			Type:             "book",
			Title:            key.title,
			Author:           key.author,
			Tags:             models.JSONStringArray{},
			Metadata:         models.JSONMap{},
		}

		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "source_document_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "author", "updated_at"}),
		}).Create(&doc).Error; err != nil {
			return nil, fmt.Errorf("upserting document: %w", err)
		}
		result.DocumentsSynced++

		// Process note shortcuts (.c1-.c5 concat, .h1-.h5 headings)
		processed := ProcessShortcuts(highlights)

		var lastHighlightedAt *time.Time
		for _, ph := range processed {
			hlTime := parseTime(ph.HighlightedAt)
			if hlTime != nil && (lastHighlightedAt == nil || hlTime.After(*lastHighlightedAt)) {
				lastHighlightedAt = hlTime
			}

			// Stable highlight ID from the processed source key
			sourceHLID := ph.SourceIDKey

			// Build tags — include heading level if set
			var tags models.JSONStringArray
			if tag := HeadingTag(ph.HeadingLevel); tag != "" {
				tags = models.JSONStringArray{tag}
			}

			highlight := models.Highlight{
				ID:                fmt.Sprintf("%s-%s", doc.ID, sanitizeID(sourceHLID)),
				DocumentID:        doc.ID,
				SourceHighlightID: sourceHLID,
				Text:              ph.Text,
				Note:              ph.Note,
				Tags:              tags,
				Location:          fmt.Sprintf("%d", ph.Location),
				LocationType:      ph.LocationType,
				LocationSortKey:   float64(ph.Location),
				HighlightedAt:     hlTime,
				SyncedAt:          time.Now(),
			}

			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "document_id"}, {Name: "source_highlight_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"text", "note", "tags", "synced_at", "updated_at"}),
			}).Create(&highlight).Error; err != nil {
				return nil, fmt.Errorf("upserting highlight: %w", err)
			}
			result.HighlightsSynced++
		}

		if lastHighlightedAt != nil {
			db.Model(&doc).Update("last_highlighted_at", lastHighlightedAt)
		}
	}

	// Update source sync time
	now := time.Now()
	db.Model(&models.Source{}).Where("id = ?", sourceID).Update("last_synced_at", &now)

	return result, nil
}

// EnsureSource creates or returns the KOReader source record.
func EnsureSource(db *gorm.DB) (*models.Source, error) {
	src := models.Source{
		ID:   "koreader",
		Type: sourceType,
		Name: "KOReader",
	}

	result := db.Where("id = ?", src.ID).FirstOrCreate(&src)
	if result.Error != nil {
		return nil, fmt.Errorf("ensuring source: %w", result.Error)
	}
	return &src, nil
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try alternative formats
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			return nil
		}
	}
	return &t
}

func sanitizeID(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", ":", "-", "'", "", "\"", "")
	result := r.Replace(strings.ToLower(s))
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}


