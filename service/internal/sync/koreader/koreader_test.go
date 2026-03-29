package koreader

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestIngest_SingleBook(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	req := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{
				Text:          "The most rewarding things in life take years",
				Title:         "How to Live",
				Author:        "Derek Sivers",
				SourceType:    "koreader",
				Category:      "books",
				Note:          "Great insight",
				Location:      42,
				LocationType:  "order",
				HighlightedAt: "2025-03-15T14:30:00Z",
			},
			{
				Text:          "Decisions are easy when you only have one choice",
				Title:         "How to Live",
				Author:        "Derek Sivers",
				SourceType:    "koreader",
				Category:      "books",
				Location:      87,
				LocationType:  "order",
				HighlightedAt: "2025-03-16T10:00:00Z",
			},
		},
	}

	result, err := Ingest(db, "koreader", req)
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	if result.DocumentsSynced != 1 {
		t.Errorf("expected 1 document, got %d", result.DocumentsSynced)
	}
	if result.HighlightsSynced != 2 {
		t.Errorf("expected 2 highlights, got %d", result.HighlightsSynced)
	}

	var doc models.Document
	if err := db.Preload("Highlights").First(&doc, "source_id = ?", "koreader").Error; err != nil {
		t.Fatalf("document not found: %v", err)
	}
	if doc.Title != "How to Live" {
		t.Errorf("unexpected title: %q", doc.Title)
	}
	if doc.Author != "Derek Sivers" {
		t.Errorf("unexpected author: %q", doc.Author)
	}
	if doc.Type != "book" {
		t.Errorf("unexpected type: %q", doc.Type)
	}
	if len(doc.Highlights) != 2 {
		t.Errorf("expected 2 highlights, got %d", len(doc.Highlights))
	}
}

func TestIngest_MultipleBooks(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	req := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Highlight from book 1", Title: "Book One", Author: "Author A", Location: 1, LocationType: "order"},
			{Text: "Highlight from book 2", Title: "Book Two", Author: "Author B", Location: 1, LocationType: "order"},
		},
	}

	result, err := Ingest(db, "koreader", req)
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	if result.DocumentsSynced != 2 {
		t.Errorf("expected 2 documents, got %d", result.DocumentsSynced)
	}

	var count int64
	db.Model(&models.Document{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 documents in DB, got %d", count)
	}
}

func TestIngest_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	req := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Same highlight", Title: "Same Book", Author: "Same Author", Location: 1, LocationType: "order"},
		},
	}

	_, _ = Ingest(db, "koreader", req)
	_, _ = Ingest(db, "koreader", req)

	var docCount, hlCount int64
	db.Model(&models.Document{}).Count(&docCount)
	db.Model(&models.Highlight{}).Count(&hlCount)

	if docCount != 1 {
		t.Errorf("expected 1 document after double ingest, got %d", docCount)
	}
	if hlCount != 1 {
		t.Errorf("expected 1 highlight after double ingest, got %d", hlCount)
	}
}

func TestIngest_UpdatesNoteOnResubmit(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	// Same text + location = same highlight, but note changes
	req1 := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Same text", Title: "Book", Author: "Author", Location: 1, LocationType: "order"},
		},
	}
	_, _ = Ingest(db, "koreader", req1)

	req2 := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Same text", Title: "Book", Author: "Author", Note: "Added a note", Location: 1, LocationType: "order"},
		},
	}
	_, _ = Ingest(db, "koreader", req2)

	var count int64
	db.Model(&models.Highlight{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 highlight (upserted), got %d", count)
	}

	var hl models.Highlight
	db.First(&hl)
	if hl.Note != "Added a note" {
		t.Errorf("expected updated note, got %q", hl.Note)
	}
}

func TestIngest_ConcatGroupGrows(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	// First ingest: two highlights in concat group 1
	req1 := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Part A", Title: "Book", Author: "Author", Note: ".c1", Location: 10, LocationType: "order"},
			{Text: "Part B", Title: "Book", Author: "Author", Note: ".c1", Location: 11, LocationType: "order"},
			{Text: "Unrelated", Title: "Book", Author: "Author", Location: 20, LocationType: "order"},
		},
	}
	_, err := Ingest(db, "koreader", req1)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	var hlCount1 int64
	db.Model(&models.Highlight{}).Count(&hlCount1)
	if hlCount1 != 2 {
		t.Fatalf("after first ingest: expected 2 highlights (1 merged + 1 solo), got %d", hlCount1)
	}

	var merged1 models.Highlight
	db.Where("source_highlight_id = ?", "concat:1").First(&merged1)
	if merged1.Text != "Part A Part B" {
		t.Errorf("first merge text = %q", merged1.Text)
	}

	// Second ingest: add a third highlight to the same concat group
	req2 := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Part A", Title: "Book", Author: "Author", Note: ".c1", Location: 10, LocationType: "order"},
			{Text: "Part B", Title: "Book", Author: "Author", Note: ".c1", Location: 11, LocationType: "order"},
			{Text: "Part C", Title: "Book", Author: "Author", Note: ".c1", Location: 12, LocationType: "order"},
			{Text: "Unrelated", Title: "Book", Author: "Author", Location: 20, LocationType: "order"},
		},
	}
	_, err = Ingest(db, "koreader", req2)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	// Should still be 2 highlights, not 3 — the concat group was updated in place
	var hlCount2 int64
	db.Model(&models.Highlight{}).Count(&hlCount2)
	if hlCount2 != 2 {
		t.Errorf("after second ingest: expected 2 highlights, got %d", hlCount2)
	}

	var merged2 models.Highlight
	db.Where("source_highlight_id = ?", "concat:1").First(&merged2)
	if merged2.Text != "Part A Part B Part C" {
		t.Errorf("second merge text = %q, want 'Part A Part B Part C'", merged2.Text)
	}

	// Same DB record (same ID)
	if merged1.ID != merged2.ID {
		t.Errorf("highlight ID changed: %q → %q", merged1.ID, merged2.ID)
	}
}

func TestIngest_HeadingStored(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	req := ReadwiseHighlightRequest{
		Highlights: []ReadwiseHighlight{
			{Text: "Chapter One", Title: "Book", Author: "Author", Note: ".h1", Location: 1, LocationType: "order"},
			{Text: "Normal highlight", Title: "Book", Author: "Author", Location: 5, LocationType: "order"},
		},
	}
	_, err := Ingest(db, "koreader", req)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	var heading models.Highlight
	db.Where("location_sort_key = ?", 1).First(&heading)
	if len(heading.Tags) != 1 || heading.Tags[0] != "h1" {
		t.Errorf("heading tags = %v, want [h1]", heading.Tags)
	}
	if heading.Note != "" {
		t.Errorf("heading note = %q, want empty (shortcode stripped)", heading.Note)
	}

	var normal models.Highlight
	db.Where("location_sort_key = ?", 5).First(&normal)
	if len(normal.Tags) != 0 {
		t.Errorf("normal tags = %v, want empty", normal.Tags)
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input string
		isNil bool
	}{
		{"2025-03-15T14:30:00Z", false},
		{"2025-03-15T14:30:00+00:00", false},
		{"", true},
		{"not-a-date", true},
	}

	for _, tt := range tests {
		got := parseTime(tt.input)
		if tt.isNil && got != nil {
			t.Errorf("parseTime(%q) = %v, want nil", tt.input, got)
		}
		if !tt.isNil && got == nil {
			t.Errorf("parseTime(%q) = nil, want non-nil", tt.input)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"How to Live:Derek Sivers", "how-to-live-derek-sivers"},
		{"Book's Title:Author", "books-title-author"},
	}

	for _, tt := range tests {
		got := sanitizeID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
