package render

import (
	"strings"
	"testing"
	"time"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

func TestRenderDocument_Book(t *testing.T) {
	r := New()
	now := time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC)
	page := 42

	doc := models.Document{
		ID:       "doc-1",
		SourceID: "src-1",
		Type:     "book",
		Title:    "How to Live",
		Author:   "Derek Sivers",
		Tags:     models.JSONStringArray{"philosophy", "life"},
		Metadata: models.JSONMap{},
		Source:   models.Source{Name: "KOReader"},
		Highlights: []models.Highlight{
			{
				Text:          "The most rewarding things in life take years",
				Note:          "Applies to software too",
				Color:         "yellow",
				Chapter:       "Master Something",
				PageNumber:    &page,
				HighlightedAt: &now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	tmpl := GetDefaultTemplate("book")
	exported, err := r.RenderDocument(doc, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exported.ID != "doc-1" {
		t.Errorf("unexpected ID: %q", exported.ID)
	}
	if exported.HighlightCount != 1 {
		t.Errorf("expected 1 highlight, got %d", exported.HighlightCount)
	}
	if !strings.Contains(exported.Content, "title:: How to Live") {
		t.Error("expected title in output")
	}
	if !strings.Contains(exported.Content, "author:: [[Derek Sivers]]") {
		t.Error("expected author in output")
	}
	if !strings.Contains(exported.Content, "The most rewarding things in life take years") {
		t.Error("expected highlight text in output")
	}
	if !strings.Contains(exported.Content, "**Note:** Applies to software too") {
		t.Error("expected note in output")
	}
	if !strings.Contains(exported.Content, "Master Something") {
		t.Error("expected chapter in output")
	}
	if !strings.Contains(exported.Content, "p. 42") {
		t.Error("expected page number in output")
	}
	if !strings.HasPrefix(exported.Checksum, "sha256:") {
		t.Errorf("expected sha256 checksum, got %q", exported.Checksum)
	}
}

func TestRenderDocument_Article(t *testing.T) {
	r := New()
	now := time.Date(2025, 3, 20, 10, 0, 0, 0, time.UTC)

	doc := models.Document{
		ID:       "doc-2",
		SourceID: "src-1",
		Type:     "article",
		Title:    "Go Testing Patterns",
		Author:   "Jane Dev",
		URL:      "https://example.com/go-testing",
		Tags:     models.JSONStringArray{"golang"},
		Metadata: models.JSONMap{"site_name": "DevBlog"},
		Source:   models.Source{Name: "Readeck"},
		Highlights: []models.Highlight{
			{
				Text:          "Table-driven tests are the gold standard",
				HighlightedAt: &now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	tmpl := GetDefaultTemplate("article")
	exported, err := r.RenderDocument(doc, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(exported.Content, "url:: https://example.com/go-testing") {
		t.Error("expected URL in output")
	}
	if !strings.Contains(exported.Content, "site:: DevBlog") {
		t.Error("expected site_name in output")
	}
	if !strings.Contains(exported.Content, "Table-driven tests are the gold standard") {
		t.Error("expected highlight text in output")
	}
}

func TestRenderDocument_EmptyHighlights(t *testing.T) {
	r := New()

	doc := models.Document{
		ID:       "doc-3",
		Type:     "book",
		Title:    "Empty Book",
		Author:   "Nobody",
		Metadata: models.JSONMap{},
		Source:   models.Source{Name: "Test"},
	}

	tmpl := GetDefaultTemplate("book")
	exported, err := r.RenderDocument(doc, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exported.HighlightCount != 0 {
		t.Errorf("expected 0 highlights, got %d", exported.HighlightCount)
	}
	if !strings.Contains(exported.Content, "## Highlights") {
		t.Error("expected Highlights heading even with no highlights")
	}
}

func TestRenderDocument_CustomTemplate(t *testing.T) {
	r := New()

	doc := models.Document{
		ID:       "doc-4",
		Type:     "book",
		Title:    "Custom",
		Author:   "Author",
		Metadata: models.JSONMap{},
		Source:   models.Source{Name: "Test"},
		Highlights: []models.Highlight{
			{Text: "First highlight"},
			{Text: "Second highlight"},
		},
	}

	tmpl := models.Template{
		PageTemplate: `# {{ title }} by {{ author }}
{% for highlight in highlights %}- {{ highlight.text }}
{% endfor %}`,
	}

	exported, err := r.RenderDocument(doc, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(exported.Content, "# Custom by Author") {
		t.Error("expected custom heading")
	}
	if !strings.Contains(exported.Content, "- First highlight") {
		t.Error("expected first highlight")
	}
	if !strings.Contains(exported.Content, "- Second highlight") {
		t.Error("expected second highlight")
	}
}

func TestRenderDocument_ChecksumStable(t *testing.T) {
	r := New()
	doc := models.Document{
		ID: "doc-5", Type: "book", Title: "Stable", Author: "A",
		Metadata: models.JSONMap{}, Source: models.Source{Name: "Test"},
	}
	tmpl := GetDefaultTemplate("book")

	e1, _ := r.RenderDocument(doc, tmpl)
	e2, _ := r.RenderDocument(doc, tmpl)

	if e1.Checksum != e2.Checksum {
		t.Error("expected identical checksums for same input")
	}
}

func TestGetDefaultTemplate(t *testing.T) {
	book := GetDefaultTemplate("book")
	if book.Type != "book" || !book.IsDefault {
		t.Error("expected default book template")
	}

	article := GetDefaultTemplate("article")
	if article.Type != "article" || !article.IsDefault {
		t.Error("expected default article template")
	}

	// Unknown types fall back to book
	unknown := GetDefaultTemplate("podcast")
	if unknown.Type != "book" {
		t.Error("expected unknown type to fall back to book template")
	}
}
