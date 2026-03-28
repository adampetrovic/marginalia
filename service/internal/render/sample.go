package render

import (
	"time"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

// SampleDocument returns a sample document for template previewing.
func SampleDocument(docType string) models.Document {
	now := time.Now()
	page := 42

	switch docType {
	case "article":
		return models.Document{
			ID:       "sample-article",
			Type:     "article",
			Title:    "Sample Article: The Future of Note-Taking",
			Author:   "Jane Smith",
			URL:      "https://example.com/future-of-notes",
			ImageURL: "https://example.com/cover.jpg",
			Tags:     models.JSONStringArray{"productivity", "notes"},
			Metadata: models.JSONMap{"site_name": "Example Blog"},
			Source:   models.Source{Name: "Readeck"},
			Highlights: []models.Highlight{
				{
					Text:          "The best notes are the ones you actually revisit",
					Note:          "This resonates with my experience",
					HighlightedAt: &now,
				},
				{
					Text:          "Linking ideas across sources creates emergent understanding",
					HighlightedAt: &now,
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
	default:
		return models.Document{
			ID:       "sample-book",
			Type:     "book",
			Title:    "Sample Book: How to Live",
			Author:   "Derek Sivers",
			ImageURL: "https://example.com/book-cover.jpg",
			Tags:     models.JSONStringArray{"philosophy", "life"},
			Metadata: models.JSONMap{},
			Source:   models.Source{Name: "KOReader"},
			Highlights: []models.Highlight{
				{
					Text:          "The most rewarding things in life take years",
					Note:          "Patience is key",
					Color:         "yellow",
					Chapter:       "Master Something",
					PageNumber:    &page,
					HighlightedAt: &now,
				},
				{
					Text:       "Decisions are easy when you only have one choice",
					Color:      "blue",
					Chapter:    "Commit",
					PageNumber: intPtr(87),
				},
			},
			CreatedAt:         now,
			UpdatedAt:         now,
			LastHighlightedAt: &now,
		}
	}
}

func intPtr(i int) *int { return &i }
