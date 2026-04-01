package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/render"
)

const testToken = "test-api-token"

func setupTestServer(t *testing.T) (*Server, *gorm.DB) {
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

	cfg := &config.Config{
		APIToken: testToken,
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	srv := NewServer(cfg, db, log)
	return srv, db
}

func doRequest(srv *Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

// --- Auth Tests ---

func TestAuth_NoToken(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v1/sources", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_WrongToken(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v1/sources", nil, "wrong-token")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v1/sources", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuth_KOReaderTokenFormat(t *testing.T) {
	srv, _ := setupTestServer(t)

	// KOReader uses "Token <token>" instead of "Bearer <token>"
	req := httptest.NewRequest("GET", "/api/v1/sources", nil)
	req.Header.Set("Authorization", "Token "+testToken)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for KOReader-style token, got %d", rr.Code)
	}
}

// --- Health ---

func TestHealthz(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/healthz", nil, "")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- Documents ---

func TestListDocuments_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v1/documents", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetDocument(t *testing.T) {
	srv, db := setupTestServer(t)

	db.Create(&models.Source{ID: "test-src", Type: "readeck", Name: "Test"})
	db.Create(&models.Document{
		ID: "doc-1", SourceID: "test-src", SourceDocumentID: "ext-1",
		Type: "article", Title: "Test Article", Author: "Author",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	db.Create(&models.Highlight{
		ID: "hl-1", DocumentID: "doc-1", SourceHighlightID: "ext-hl-1",
		Text: "Important text",
	})

	rr := doRequest(srv, "GET", "/api/v1/documents/doc-1", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var doc models.Document
	_ = json.NewDecoder(rr.Body).Decode(&doc)
	if doc.Title != "Test Article" {
		t.Errorf("unexpected title: %q", doc.Title)
	}
	if len(doc.Highlights) != 1 {
		t.Errorf("expected 1 highlight, got %d", len(doc.Highlights))
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v1/documents/nonexistent", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- Templates ---

func TestTemplateCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create
	tmpl := models.Template{
		ID:           "tmpl-1",
		Name:         "My Book Template",
		Type:         "book",
		PageTemplate: "# {{ title }}\n{% for h in highlights %}- {{ h.text }}\n{% endfor %}",
		IsDefault:    true,
	}
	rr := doRequest(srv, "POST", "/api/v1/templates", tmpl, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// List
	rr = doRequest(srv, "GET", "/api/v1/templates", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}
	var templates []models.Template
	_ = json.NewDecoder(rr.Body).Decode(&templates)
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}

	// Get
	rr = doRequest(srv, "GET", "/api/v1/templates/tmpl-1", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rr.Code)
	}

	// Update
	update := models.Template{
		Name:         "Updated Template",
		PageTemplate: "## {{ title }}",
	}
	rr = doRequest(srv, "PUT", "/api/v1/templates/tmpl-1", update, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var updated models.Template
	_ = json.NewDecoder(rr.Body).Decode(&updated)
	if updated.Name != "Updated Template" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}
}

func TestPreviewTemplate(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := map[string]string{
		"page_template": "# {{ title }} by {{ author }}",
		"type":          "book",
	}
	rr := doRequest(srv, "POST", "/api/v1/templates/preview", body, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var exported render.ExportedDocument
	_ = json.NewDecoder(rr.Body).Decode(&exported)
	if exported.Content == "" {
		t.Error("expected non-empty rendered content")
	}
}

// --- Export ---

func TestExport(t *testing.T) {
	srv, db := setupTestServer(t)

	db.Create(&models.Source{ID: "test-src", Type: "readeck", Name: "Readeck"})
	db.Create(&models.Document{
		ID: "doc-1", SourceID: "test-src", SourceDocumentID: "ext-1",
		Type: "article", Title: "Test Article", Author: "Author",
		Tags: models.JSONStringArray{"tech"}, Metadata: models.JSONMap{"site_name": "Blog"},
	})
	db.Create(&models.Highlight{
		ID: "hl-1", DocumentID: "doc-1", SourceHighlightID: "ext-hl-1",
		Text: "Key insight from the article",
	})

	rr := doRequest(srv, "GET", "/api/v1/export", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var exported []*render.ExportedDocument
	_ = json.NewDecoder(rr.Body).Decode(&exported)
	if len(exported) != 1 {
		t.Fatalf("expected 1 exported doc, got %d", len(exported))
	}
	if exported[0].Content == "" {
		t.Error("expected non-empty content")
	}
	if exported[0].Checksum == "" {
		t.Error("expected checksum")
	}
}

// --- Readwise-compatible (KOReader) ---

func TestReadwiseHighlights(t *testing.T) {
	srv, db := setupTestServer(t)

	body := map[string]interface{}{
		"highlights": []map[string]interface{}{
			{
				"text":           "Test highlight",
				"title":          "Test Book",
				"author":         "Test Author",
				"source_type":    "koreader",
				"category":       "books",
				"location":       42,
				"location_type":  "order",
				"highlighted_at": "2025-03-15T14:30:00Z",
			},
		},
	}

	rr := doRequest(srv, "POST", "/api/v2/highlights", body, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify data stored
	var count int64
	db.Model(&models.Document{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
	db.Model(&models.Highlight{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 highlight, got %d", count)
	}
}

func TestReadwiseAuth(t *testing.T) {
	srv, _ := setupTestServer(t)
	rr := doRequest(srv, "GET", "/api/v2/auth", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
