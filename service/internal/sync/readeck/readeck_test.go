package readeck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestClient_GetBookmarks(t *testing.T) {
	bookmarks := []Bookmark{
		{ID: "bm1", Title: "Test Article", URL: "https://example.com/article", Authors: []string{"Author One"}, Labels: []string{"tech"}, SiteName: "Example"},
		{ID: "bm2", Title: "Another Article", URL: "https://example.com/another"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(bookmarks)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	got, err := client.GetBookmarks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 bookmarks, got %d", len(got))
	}
	if got[0].Title != "Test Article" {
		t.Errorf("unexpected title: %q", got[0].Title)
	}
}

func TestClient_GetAnnotations(t *testing.T) {
	annotations := []Annotation{
		{ID: "ann1", Text: "First highlight"},
		{ID: "ann2", Text: "Second highlight"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/bookmarks/bm1/annotations" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(annotations)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	got, err := client.GetAnnotations("bm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(got))
	}
}

func TestClient_UnauthorizedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "wrong-token")
	_, err := client.GetBookmarks()
	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestSyncer_Sync(t *testing.T) {
	bookmarks := []Bookmark{
		{
			ID:       "bm1",
			Title:    "Test Article",
			URL:      "https://example.com/article",
			Authors:  []string{"Jane Doe"},
			Labels:   []string{"tech", "go"},
			SiteName: "Example Blog",
		},
		{
			ID:    "bm2",
			Title: "No Highlights Article",
			URL:   "https://example.com/empty",
		},
	}

	annotations := map[string][]Annotation{
		"bm1": {
			{ID: "ann1", Text: "This is an important point"},
			{ID: "ann2", Text: "Another key insight"},
		},
		"bm2": {}, // No annotations
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/bookmarks":
			_ = json.NewEncoder(w).Encode(bookmarks)
		case "/api/bookmarks/bm1/annotations":
			_ = json.NewEncoder(w).Encode(annotations["bm1"])
		case "/api/bookmarks/bm2/annotations":
			_ = json.NewEncoder(w).Encode(annotations["bm2"])
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db := setupTestDB(t)
	db.Create(&models.Source{ID: "readeck", Type: "readeck", Name: "Readeck"})

	client := NewClient(srv.URL, "test-token")
	syncer := NewSyncer(client, db)

	result, err := syncer.Sync("readeck")
	if err != nil {
		t.Fatalf("sync error: %v", err)
	}

	// Only bm1 has annotations, so only 1 document synced
	if result.DocumentsSynced != 1 {
		t.Errorf("expected 1 document synced, got %d", result.DocumentsSynced)
	}
	if result.HighlightsSynced != 2 {
		t.Errorf("expected 2 highlights synced, got %d", result.HighlightsSynced)
	}

	// Verify document in DB
	var doc models.Document
	if err := db.Preload("Highlights").First(&doc, "source_document_id = ?", "bm1").Error; err != nil {
		t.Fatalf("document not found: %v", err)
	}
	if doc.Title != "Test Article" {
		t.Errorf("unexpected title: %q", doc.Title)
	}
	if doc.Author != "Jane Doe" {
		t.Errorf("unexpected author: %q", doc.Author)
	}
	if len(doc.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(doc.Tags))
	}
	if len(doc.Highlights) != 2 {
		t.Errorf("expected 2 highlights, got %d", len(doc.Highlights))
	}
}

func TestSyncer_Sync_Idempotent(t *testing.T) {
	bookmarks := []Bookmark{
		{ID: "bm1", Title: "Test", URL: "https://example.com", Authors: []string{"A"}, SiteName: "Ex"},
	}
	annotations := []Annotation{
		{ID: "ann1", Text: "Highlight text"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/bookmarks":
			_ = json.NewEncoder(w).Encode(bookmarks)
		case "/api/bookmarks/bm1/annotations":
			_ = json.NewEncoder(w).Encode(annotations)
		}
	}))
	defer srv.Close()

	db := setupTestDB(t)
	db.Create(&models.Source{ID: "readeck", Type: "readeck", Name: "Readeck"})

	client := NewClient(srv.URL, "test-token")
	syncer := NewSyncer(client, db)

	// Sync twice
	_, _ = syncer.Sync("readeck")
	_, _ = syncer.Sync("readeck")

	// Should still only have 1 document and 1 highlight
	var docCount, hlCount int64
	db.Model(&models.Document{}).Count(&docCount)
	db.Model(&models.Highlight{}).Count(&hlCount)

	if docCount != 1 {
		t.Errorf("expected 1 document after double sync, got %d", docCount)
	}
	if hlCount != 1 {
		t.Errorf("expected 1 highlight after double sync, got %d", hlCount)
	}
}

func TestSyncer_Sync_UpdatesExisting(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/bookmarks":
			title := "Original Title"
			if callCount > 0 {
				title = "Updated Title"
			}
			callCount++
			_ = json.NewEncoder(w).Encode([]Bookmark{
				{ID: "bm1", Title: title, URL: "https://example.com", SiteName: "Ex"},
			})
		case "/api/bookmarks/bm1/annotations":
			_ = json.NewEncoder(w).Encode([]Annotation{
				{ID: "ann1", Text: "Some text"},
			})
		}
	}))
	defer srv.Close()

	db := setupTestDB(t)
	db.Create(&models.Source{ID: "readeck", Type: "readeck", Name: "Readeck"})

	client := NewClient(srv.URL, "test-token")
	syncer := NewSyncer(client, db)

	_, _ = syncer.Sync("readeck")
	_, _ = syncer.Sync("readeck")

	var doc models.Document
	db.First(&doc, "source_document_id = ?", "bm1")
	if doc.Title != "Updated Title" {
		t.Errorf("expected updated title, got %q", doc.Title)
	}
}

func TestEnsureSource(t *testing.T) {
	db := setupTestDB(t)

	src, err := EnsureSource(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.ID != "readeck" {
		t.Errorf("unexpected source ID: %q", src.ID)
	}

	// Calling again should return the same source
	src2, err := EnsureSource(db)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if src2.ID != src.ID {
		t.Error("expected same source on second call")
	}
}
