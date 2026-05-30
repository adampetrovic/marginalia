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

	"github.com/adampetrovic/marginalia/service/internal/auth"
	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/render"
)

const (
	testToken  = "test-api-token"
	testUserID = "user-test"
)

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

	// Seed a test user and a personal API token whose plaintext is testToken.
	if err := db.Create(&models.User{
		ID:           testUserID,
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: mustHash(t, "password123"),
		IsAdmin:      true,
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&models.APIToken{
		ID:        "tok-test",
		UserID:    testUserID,
		Name:      "Test token",
		TokenHash: auth.HashAPIToken(testToken),
		Prefix:    testToken[:8],
	}).Error; err != nil {
		t.Fatalf("failed to seed token: %v", err)
	}

	cfg := &config.Config{SessionSecret: "test-session-secret"}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	srv := NewServer(cfg, db, log)
	return srv, db
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	return h
}

// seedDoc creates a document (and optional highlight) owned by the test user.
func seedDoc(t *testing.T, db *gorm.DB, docID, hlID string) {
	t.Helper()
	db.Create(&models.Source{ID: "test-src", UserID: testUserID, Type: "readeck", Name: "Test"})
	db.Create(&models.Document{
		ID: docID, UserID: testUserID, SourceID: "test-src", SourceDocumentID: "ext-" + docID,
		Type: "article", Title: "Test Article", Author: "Author",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	if hlID != "" {
		db.Create(&models.Highlight{
			ID: hlID, UserID: testUserID, DocumentID: docID, SourceHighlightID: "ext-" + hlID,
			Text: "Important text",
		})
	}
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

// reviewCardResp mirrors the JSON shape returned by the review endpoints.
type reviewCardResp struct {
	Done  bool `json:"done"`
	Stats struct {
		Due           int64 `json:"due"`
		New           int64 `json:"new"`
		ReviewedToday int64 `json:"reviewed_today"`
	} `json:"stats"`
	Highlight *models.Highlight `json:"highlight"`
	Document  *models.Document  `json:"document"`
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

// --- Registration / login / session ---

func TestRegisterLoginAndSession(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Register a brand-new account.
	rr := doRequest(srv, "POST", "/api/v1/auth/register",
		map[string]string{"email": "new@example.com", "password": "supersecret", "name": "New"}, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	cookie := sessionCookieFrom(rr)
	if cookie == "" {
		t.Fatal("register did not set a session cookie")
	}

	// The session cookie should authenticate /me.
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Cookie", sessionCookie+"="+cookie)
	meRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(meRR, req)
	if meRR.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d: %s", meRR.Code, meRR.Body.String())
	}

	// Login with the same credentials.
	rr = doRequest(srv, "POST", "/api/v1/auth/login",
		map[string]string{"email": "new@example.com", "password": "supersecret"}, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Wrong password is rejected.
	rr = doRequest(srv, "POST", "/api/v1/auth/login",
		map[string]string{"email": "new@example.com", "password": "nope"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", rr.Code)
	}
}

func sessionCookieFrom(rr *httptest.ResponseRecorder) string {
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	return ""
}

// --- API tokens ---

func TestTokenLifecycle(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doRequest(srv, "POST", "/api/v1/tokens", map[string]string{"name": "My device"}, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create token: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&created)
	if created.Token == "" {
		t.Fatal("expected plaintext token in response")
	}

	// The new token should authenticate.
	rr = doRequest(srv, "GET", "/api/v1/sources", nil, created.Token)
	if rr.Code != http.StatusOK {
		t.Fatalf("new token auth: expected 200, got %d", rr.Code)
	}

	// Revoke it; afterwards it must not authenticate.
	rr = doRequest(srv, "DELETE", "/api/v1/tokens/"+created.ID, nil, testToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete token: expected 204, got %d", rr.Code)
	}
	rr = doRequest(srv, "GET", "/api/v1/sources", nil, created.Token)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: expected 401, got %d", rr.Code)
	}
}

// --- User scoping ---

func TestDocumentsAreUserScoped(t *testing.T) {
	srv, db := setupTestServer(t)

	// A document owned by a different user must be invisible.
	db.Create(&models.User{ID: "user-other", Email: "other@example.com", PasswordHash: mustHash(t, "password123")})
	db.Create(&models.Source{ID: "other-src", UserID: "user-other", Type: "readeck", Name: "Other"})
	db.Create(&models.Document{
		ID: "other-doc", UserID: "user-other", SourceID: "other-src", SourceDocumentID: "x",
		Type: "article", Title: "Secret", Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})

	rr := doRequest(srv, "GET", "/api/v1/documents", nil, testToken)
	var docs []models.Document
	_ = json.NewDecoder(rr.Body).Decode(&docs)
	if len(docs) != 0 {
		t.Errorf("expected 0 documents for test user, got %d", len(docs))
	}

	rr = doRequest(srv, "GET", "/api/v1/documents/other-doc", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for another user's document, got %d", rr.Code)
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
	seedDoc(t, db, "doc-1", "hl-1")

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

// TestStatsDueReviews guards against double-counting: a brand-new highlight is
// both "new" and "due", so due_reviews must equal the number of highlights, not
// twice that.
func TestStatsDueReviews(t *testing.T) {
	srv, db := setupTestServer(t)
	db.Create(&models.Source{ID: "test-src", UserID: testUserID, Type: "koreader", Name: "Test"})
	db.Create(&models.Document{ID: "doc-1", UserID: testUserID, SourceID: "test-src", SourceDocumentID: "d1", Type: "book", Title: "B", Tags: models.JSONStringArray{}, Metadata: models.JSONMap{}})
	for _, id := range []string{"h1", "h2", "h3"} {
		db.Create(&models.Highlight{ID: id, UserID: testUserID, DocumentID: "doc-1", SourceHighlightID: id, Text: "t"})
	}

	rr := doRequest(srv, "GET", "/api/v1/stats", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var stats struct {
		Books      int64 `json:"books"`
		Highlights int64 `json:"highlights"`
		DueReviews int64 `json:"due_reviews"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&stats)
	if stats.Books != 1 {
		t.Errorf("expected 1 book, got %d", stats.Books)
	}
	if stats.Highlights != 3 {
		t.Errorf("expected 3 highlights, got %d", stats.Highlights)
	}
	if stats.DueReviews != 3 {
		t.Errorf("expected 3 due reviews (not double-counted), got %d", stats.DueReviews)
	}
}

// --- Review ---

func TestReviewQueueIncludesNewHighlights(t *testing.T) {
	srv, db := setupTestServer(t)
	seedDoc(t, db, "doc-1", "hl-1")

	rr := doRequest(srv, "GET", "/api/v1/review", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var res reviewCardResp
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res.Stats.Due != 1 {
		t.Errorf("expected 1 due highlight, got %d", res.Stats.Due)
	}
	if res.Stats.New != 1 {
		t.Errorf("expected 1 new highlight, got %d", res.Stats.New)
	}
	if res.Highlight == nil || res.Highlight.ID != "hl-1" {
		t.Fatalf("expected hl-1 as next review, got %#v", res.Highlight)
	}
	if res.Document == nil || res.Document.ID != "doc-1" {
		t.Fatalf("expected document context, got %#v", res.Document)
	}
}

func TestReviewActionSchedulesHighlight(t *testing.T) {
	srv, db := setupTestServer(t)
	seedDoc(t, db, "doc-1", "hl-1")

	rr := doRequest(srv, "POST", "/api/v1/review/hl-1", map[string]string{"rating": "good"}, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// The action response is the next card; the queue should now be empty.
	var res reviewCardResp
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if !res.Done {
		t.Errorf("expected queue done after the only highlight was scheduled")
	}

	// Verify the persisted schedule.
	var state models.ReviewState
	if err := db.First(&state, "highlight_id = ?", "hl-1").Error; err != nil {
		t.Fatalf("expected review state: %v", err)
	}
	if state.IntervalDays != 1 {
		t.Errorf("first good review should be due in 1 day, got %d", state.IntervalDays)
	}
	if state.Repetitions != 1 {
		t.Errorf("expected 1 repetition, got %d", state.Repetitions)
	}
	if state.DueAt == nil {
		t.Fatal("expected due date")
	}
	if state.UserID != testUserID {
		t.Errorf("expected review state scoped to user, got %q", state.UserID)
	}
}

func TestReviewActionArchivesHighlight(t *testing.T) {
	srv, db := setupTestServer(t)
	seedDoc(t, db, "doc-1", "hl-1")

	rr := doRequest(srv, "POST", "/api/v1/review/hl-1", map[string]string{"rating": "archive"}, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var state models.ReviewState
	if err := db.First(&state, "highlight_id = ?", "hl-1").Error; err != nil {
		t.Fatalf("expected review state: %v", err)
	}
	if !state.Suspended {
		t.Error("expected archived review state to be suspended")
	}
	if state.DueAt != nil {
		t.Error("expected archived review state to have no due date")
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
	var createdTmpl models.Template
	_ = json.NewDecoder(rr.Body).Decode(&createdTmpl)
	if createdTmpl.UserID != testUserID {
		t.Errorf("expected template scoped to user, got %q", createdTmpl.UserID)
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
	rr = doRequest(srv, "GET", "/api/v1/templates/"+createdTmpl.ID, nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rr.Code)
	}

	// Update
	update := models.Template{
		Name:         "Updated Template",
		Type:         "book",
		PageTemplate: "## {{ title }}",
	}
	rr = doRequest(srv, "PUT", "/api/v1/templates/"+createdTmpl.ID, update, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var updated models.Template
	_ = json.NewDecoder(rr.Body).Decode(&updated)
	if updated.Name != "Updated Template" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}

	// Delete
	rr = doRequest(srv, "DELETE", "/api/v1/templates/"+createdTmpl.ID, nil, testToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rr.Code)
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

	db.Create(&models.Source{ID: "test-src", UserID: testUserID, Type: "readeck", Name: "Readeck"})
	db.Create(&models.Document{
		ID: "doc-1", UserID: testUserID, SourceID: "test-src", SourceDocumentID: "ext-1",
		Type: "article", Title: "Test Article", Author: "Author",
		Tags: models.JSONStringArray{"tech"}, Metadata: models.JSONMap{"site_name": "Blog"},
	})
	db.Create(&models.Highlight{
		ID: "hl-1", UserID: testUserID, DocumentID: "doc-1", SourceHighlightID: "ext-hl-1",
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

	// Verify data stored and scoped to the token's user.
	var count int64
	db.Model(&models.Document{}).Where("user_id = ?", testUserID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
	db.Model(&models.Highlight{}).Where("user_id = ?", testUserID).Count(&count)
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

// Readest uses a custom base URL of ".../api/v2" and appends "/auth/" and
// "/highlights/" with a trailing slash, so both forms must be accepted.
func TestReadwiseEndpoints_TrailingSlash(t *testing.T) {
	srv, db := setupTestServer(t)

	rr := doRequest(srv, "GET", "/api/v2/auth/", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Errorf("auth: expected 200, got %d", rr.Code)
	}

	body := map[string]interface{}{
		"highlights": []map[string]interface{}{
			{
				"text":           "Reading on an e-ink screen is calmer.",
				"title":          "Deep Work",
				"author":         "Cal Newport",
				"source_type":    "readest",
				"category":       "books",
				"location":       12,
				"location_type":  "order",
				"highlighted_at": "2026-05-20T10:00:00Z",
			},
		},
	}

	rr = doRequest(srv, "POST", "/api/v2/highlights/", body, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("highlights: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int64
	db.Model(&models.Highlight{}).Where("user_id = ?", testUserID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 highlight, got %d", count)
	}
}
