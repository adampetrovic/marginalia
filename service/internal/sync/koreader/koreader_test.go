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
