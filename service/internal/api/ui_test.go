package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

func seedTestData(t *testing.T, srv *Server) {
	t.Helper()
	srv.db.Create(&models.Source{ID: "test-src", Type: "readeck", Name: "Test"})
	srv.db.Create(&models.Source{ID: "koreader", Type: "koreader", Name: "KOReader"})

	srv.db.Create(&models.Document{
		ID: "book-1", SourceID: "koreader", SourceDocumentID: "b1",
		Type: "book", Title: "How to Live", Author: "Derek Sivers",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	srv.db.Create(&models.Document{
		ID: "book-2", SourceID: "koreader", SourceDocumentID: "b2",
		Type: "book", Title: "Meditations", Author: "Marcus Aurelius",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	srv.db.Create(&models.Document{
		ID: "article-1", SourceID: "test-src", SourceDocumentID: "a1",
		Type: "article", Title: "End of Productivity Theater", Author: "Murat",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	srv.db.Create(&models.Document{
		ID: "article-2", SourceID: "test-src", SourceDocumentID: "a2",
		Type: "article", Title: "What Is Anthropic Thinking?", Author: "Derek Thompson",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})

	srv.db.Create(&models.Highlight{
		ID: "hl-1", DocumentID: "book-1", SourceHighlightID: "h1",
		Text: "The most rewarding things in life take years",
		Note: "Patience matters",
	})
	srv.db.Create(&models.Highlight{
		ID: "hl-2", DocumentID: "book-1", SourceHighlightID: "h2",
		Text: "Be useful to others",
	})
	srv.db.Create(&models.Highlight{
		ID: "hl-3", DocumentID: "book-2", SourceHighlightID: "h3",
		Text: "You have power over your mind",
		Note: "Stoicism",
	})
	srv.db.Create(&models.Highlight{
		ID: "hl-4", DocumentID: "article-1", SourceHighlightID: "h4",
		Text: "Productivity should be about working on the right things",
	})
}

func doUIRequest(srv *Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func doUIPost(srv *Server, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

// --- Dashboard renders ---

func TestUI_Dashboard_Renders(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()

	// Stats present
	if !strings.Contains(body, ">2<") { // 2 books
		t.Error("expected book count of 2")
	}
	if !strings.Contains(body, "How to Live") {
		t.Error("expected 'How to Live' in document list")
	}
	if !strings.Contains(body, "Meditations") {
		t.Error("expected 'Meditations' in document list")
	}
}

// --- Tab filtering ---

func TestUI_Dashboard_TabAll(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?tab=all")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "How to Live") {
		t.Error("tab=all should show books")
	}
	if !strings.Contains(body, "End of Productivity") {
		t.Error("tab=all should show articles")
	}
}

func TestUI_Dashboard_TabBook(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?tab=book")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "How to Live") {
		t.Error("tab=book should show books")
	}
	if strings.Contains(body, "End of Productivity") {
		t.Error("tab=book should NOT show articles")
	}
}

func TestUI_Dashboard_TabArticle(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?tab=article")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "How to Live") {
		t.Error("tab=article should NOT show books")
	}
	if !strings.Contains(body, "End of Productivity") {
		t.Error("tab=article should show articles")
	}
}

// --- Title search ---

func TestUI_Dashboard_SearchTitle(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?q=Meditations")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Meditations") {
		t.Error("search should return Meditations")
	}
	if strings.Contains(body, "How to Live") {
		t.Error("search should NOT return How to Live")
	}
}

func TestUI_Dashboard_SearchAuthor(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?q=Derek")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// "Derek Sivers" (book) and "Derek Thompson" (article)
	if !strings.Contains(body, "How to Live") {
		t.Error("search by author should return Derek Sivers' book")
	}
	if !strings.Contains(body, "Anthropic") {
		t.Error("search by author should return Derek Thompson's article")
	}
}

func TestUI_Dashboard_SearchNoResults(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?q=nonexistent")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No documents matching") {
		t.Error("expected empty state message for no results")
	}
}

func TestUI_Dashboard_SearchWithTab(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	// Search "Derek" but only in books tab
	rr := doUIRequest(srv, "/?q=Derek&tab=book")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "How to Live") {
		t.Error("search+tab=book should return Derek Sivers' book")
	}
	if strings.Contains(body, "Anthropic") {
		t.Error("search+tab=book should NOT return Derek Thompson's article")
	}
}

// --- Highlight search ---

func TestUI_Dashboard_HighlightSearch(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?hl=rewarding")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "rewarding things in life") {
		t.Error("highlight search should find matching text")
	}
	if !strings.Contains(body, "How to Live") {
		t.Error("highlight search should show parent document title")
	}
}

func TestUI_Dashboard_HighlightSearchByNote(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?hl=Stoicism")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "power over your mind") {
		t.Error("highlight search by note should find matching highlight")
	}
}

func TestUI_Dashboard_HighlightSearchNoResults(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/?hl=zzzznotfound")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No highlights matching") {
		t.Error("expected empty state for no highlight results")
	}
}

// --- Review ---

func TestUI_Review_RendersDueHighlight(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/review")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Daily Review") {
		t.Error("review page should show title")
	}
	if !strings.Contains(body, "The most rewarding things in life take years") {
		t.Error("review page should show a due highlight")
	}
	if !strings.Contains(body, "Again") || !strings.Contains(body, "Good") || !strings.Contains(body, "Easy") {
		t.Error("review page should show rating buttons")
	}
}

func TestUI_Review_ActionAdvancesQueue(t *testing.T) {
	srv, db := setupTestServer(t)

	srv.db.Create(&models.Source{ID: "test-src", Type: "readeck", Name: "Test"})
	srv.db.Create(&models.Document{
		ID: "doc-1", SourceID: "test-src", SourceDocumentID: "a1",
		Type: "article", Title: "A useful article", Author: "Murat",
		Tags: models.JSONStringArray{}, Metadata: models.JSONMap{},
	})
	srv.db.Create(&models.Highlight{ID: "hl-1", DocumentID: "doc-1", SourceHighlightID: "h1", Text: "First idea"})
	srv.db.Create(&models.Highlight{ID: "hl-2", DocumentID: "doc-1", SourceHighlightID: "h2", Text: "Second idea"})

	rr := doUIPost(srv, "/review/hl-1", url.Values{"rating": {"good"}})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Review saved") {
		t.Error("expected review action confirmation")
	}
	if strings.Contains(body, "First idea") {
		t.Error("scheduled highlight should leave the due queue")
	}
	if !strings.Contains(body, "Second idea") {
		t.Error("next due highlight should be shown")
	}

	var state models.ReviewState
	if err := db.First(&state, "highlight_id = ?", "hl-1").Error; err != nil {
		t.Fatalf("expected review state: %v", err)
	}
	if state.IntervalDays != 1 {
		t.Errorf("expected interval 1, got %d", state.IntervalDays)
	}
}

// --- Document detail ---

func TestUI_DocumentDetail(t *testing.T) {
	srv, _ := setupTestServer(t)
	seedTestData(t, srv)

	rr := doUIRequest(srv, "/documents/book-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "How to Live") {
		t.Error("document detail should show title")
	}
	if !strings.Contains(body, "rewarding things") {
		t.Error("document detail should show highlights")
	}
	if !strings.Contains(body, "Patience matters") {
		t.Error("document detail should show notes")
	}
}

func TestUI_DocumentDetail_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doUIRequest(srv, "/documents/nonexistent")
	// Should redirect to dashboard
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
}
