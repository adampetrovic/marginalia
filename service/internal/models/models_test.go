package models

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestAutoMigrate(t *testing.T) {
	db := setupTestDB(t)

	// Verify all tables exist
	tables := []string{"sources", "documents", "highlights", "review_states", "templates", "sync_logs"}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("expected table %q to exist", table)
		}
	}
}

func TestSourceCRUD(t *testing.T) {
	db := setupTestDB(t)

	src := Source{
		ID:   "readeck-1",
		Type: "readeck",
		Name: "My Readeck",
	}
	if err := db.Create(&src).Error; err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	var got Source
	if err := db.First(&got, "id = ?", "readeck-1").Error; err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if got.Name != "My Readeck" {
		t.Errorf("expected name 'My Readeck', got %q", got.Name)
	}
}

func TestDocumentWithJSONFields(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&Source{ID: "src-1", Type: "readeck", Name: "Test"})

	doc := Document{
		ID:               "doc-1",
		SourceID:         "src-1",
		SourceDocumentID: "ext-1",
		Type:             "article",
		Title:            "Test Article",
		Author:           "Test Author",
		Tags:             JSONStringArray{"go", "testing"},
		Metadata:         JSONMap{"language": "en", "word_count": float64(1500)},
	}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	var got Document
	if err := db.First(&got, "id = ?", "doc-1").Error; err != nil {
		t.Fatalf("failed to read document: %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Errorf("unexpected tags: %v", got.Tags)
	}
	if got.Metadata["language"] != "en" {
		t.Errorf("unexpected metadata: %v", got.Metadata)
	}
}

func TestDocumentDeduplication(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&Source{ID: "src-1", Type: "readeck", Name: "Test"})

	doc1 := Document{ID: "doc-1", SourceID: "src-1", SourceDocumentID: "ext-1", Type: "article", Title: "First"}
	doc2 := Document{ID: "doc-2", SourceID: "src-1", SourceDocumentID: "ext-1", Type: "article", Title: "Duplicate"}

	db.Create(&doc1)
	err := db.Create(&doc2).Error
	if err == nil {
		t.Error("expected unique constraint violation for duplicate source_document_id")
	}
}

func TestHighlightBelongsToDocument(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&Source{ID: "src-1", Type: "readeck", Name: "Test"})
	db.Create(&Document{ID: "doc-1", SourceID: "src-1", SourceDocumentID: "ext-1", Type: "book", Title: "Test Book"})

	now := time.Now()
	hl := Highlight{
		ID:                "hl-1",
		DocumentID:        "doc-1",
		SourceHighlightID: "ext-hl-1",
		Text:              "This is a highlighted passage",
		Note:              "Great insight",
		Color:             "yellow",
		Chapter:           "Chapter 1",
		PageNumber:        intPtr(42),
		HighlightedAt:     &now,
		SyncedAt:          now,
	}
	if err := db.Create(&hl).Error; err != nil {
		t.Fatalf("failed to create highlight: %v", err)
	}

	// Load document with highlights
	var doc Document
	if err := db.Preload("Highlights").First(&doc, "id = ?", "doc-1").Error; err != nil {
		t.Fatalf("failed to load document: %v", err)
	}
	if len(doc.Highlights) != 1 {
		t.Errorf("expected 1 highlight, got %d", len(doc.Highlights))
	}
	if doc.Highlights[0].Text != "This is a highlighted passage" {
		t.Errorf("unexpected highlight text: %q", doc.Highlights[0].Text)
	}
}

func TestHighlightDeduplication(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&Source{ID: "src-1", Type: "readeck", Name: "Test"})
	db.Create(&Document{ID: "doc-1", SourceID: "src-1", SourceDocumentID: "ext-1", Type: "book", Title: "Test"})

	hl1 := Highlight{ID: "hl-1", DocumentID: "doc-1", SourceHighlightID: "ext-hl-1", Text: "First", SyncedAt: time.Now()}
	hl2 := Highlight{ID: "hl-2", DocumentID: "doc-1", SourceHighlightID: "ext-hl-1", Text: "Duplicate", SyncedAt: time.Now()}

	db.Create(&hl1)
	err := db.Create(&hl2).Error
	if err == nil {
		t.Error("expected unique constraint violation for duplicate source_highlight_id within document")
	}
}

func TestReviewStateBelongsToHighlight(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&Source{ID: "src-1", Type: "readeck", Name: "Test"})
	db.Create(&Document{ID: "doc-1", SourceID: "src-1", SourceDocumentID: "ext-1", Type: "book", Title: "Test Book"})
	db.Create(&Highlight{ID: "hl-1", DocumentID: "doc-1", SourceHighlightID: "ext-hl-1", Text: "A durable idea", SyncedAt: time.Now()})

	due := time.Now().AddDate(0, 0, 3)
	state := ReviewState{HighlightID: "hl-1", EaseFactor: 2.5, IntervalDays: 3, Repetitions: 2, DueAt: &due}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("failed to create review state: %v", err)
	}

	var hl Highlight
	if err := db.Preload("ReviewState").First(&hl, "id = ?", "hl-1").Error; err != nil {
		t.Fatalf("failed to load highlight: %v", err)
	}
	if hl.ReviewState == nil {
		t.Fatal("expected review state to preload")
	}
	if hl.ReviewState.IntervalDays != 3 {
		t.Errorf("expected interval 3, got %d", hl.ReviewState.IntervalDays)
	}
}

func TestTemplateCRUD(t *testing.T) {
	db := setupTestDB(t)

	tmpl := Template{
		ID:                "tmpl-1",
		Name:              "Default Book",
		Type:              "book",
		PageTemplate:      "title:: {{ title }}",
		HighlightTemplate: "> {{ text }}",
		IsDefault:         true,
	}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	var got Template
	db.First(&got, "id = ?", "tmpl-1")
	if !got.IsDefault {
		t.Error("expected template to be default")
	}
}

func intPtr(i int) *int { return &i }
