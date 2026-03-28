package render

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

// DefaultBookPageTemplate is the built-in template for book documents.
const DefaultBookPageTemplate = `title:: {{ title }}
author:: [[{{ author }}]]
category:: #{{ category }}
source:: {{ source }}
{% if image_url %}cover:: {{ image_url }}
{% endif %}{% if last_highlighted_at %}last_highlighted:: {{ last_highlighted_at }}
{% endif %}{% if tags %}tags:: {% for tag in tags %}#{{ tag }} {% endfor %}
{% endif %}
- ## Highlights
{% for highlight in highlights %}
	- > {{ highlight.text }}
{% if highlight.note %}		- **Note:** {{ highlight.note }}
{% endif %}{% if highlight.color %}		- color:: {{ highlight.color }}
{% endif %}{% if highlight.chapter or highlight.page_number %}		- location::{% if highlight.chapter %} {{ highlight.chapter }}{% endif %}{% if highlight.page_number %} (p. {{ highlight.page_number }}){% endif %}
{% endif %}{% endfor %}`

// DefaultArticlePageTemplate is the built-in template for article documents.
const DefaultArticlePageTemplate = `title:: {{ title }}
author:: {{ author }}
category:: #article
url:: {{ url }}
{% if site_name %}site:: {{ site_name }}
{% endif %}{% if created_at %}saved:: {{ created_at }}
{% endif %}{% if tags %}labels:: {% for tag in tags %}#{{ tag }} {% endfor %}
{% endif %}
- ## Highlights
{% for highlight in highlights %}
	- > {{ highlight.text }}
{% if highlight.note %}		- **Note:** {{ highlight.note }}
{% endif %}{% endfor %}`

// ExportedDocument is the result of rendering a document through its template.
type ExportedDocument struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	Content        string `json:"content"`
	UpdatedAt      string `json:"updated_at"`
	HighlightCount int    `json:"highlight_count"`
	Checksum       string `json:"checksum"`
}

// Renderer renders documents using pongo2 templates.
type Renderer struct{}

// New creates a new Renderer.
func New() *Renderer {
	return &Renderer{}
}

// RenderDocument renders a document and its highlights using the given template.
func (r *Renderer) RenderDocument(doc models.Document, tmpl models.Template) (*ExportedDocument, error) {
	tpl, err := pongo2.FromString(tmpl.PageTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	highlights := make([]pongo2.Context, 0, len(doc.Highlights))
	for _, hl := range doc.Highlights {
		hlCtx := pongo2.Context{
			"text":           hl.Text,
			"note":           hl.Note,
			"color":          hl.Color,
			"chapter":        hl.Chapter,
			"page_number":    hl.PageNumber,
			"percentage":     hl.Percentage,
			"location":       hl.Location,
			"highlighted_at": formatTime(hl.HighlightedAt),
			"tags":           []string(hl.Tags),
		}
		highlights = append(highlights, hlCtx)
	}

	ctx := pongo2.Context{
		"title":              doc.Title,
		"author":             doc.Author,
		"category":           doc.Type,
		"type":               doc.Type,
		"url":                doc.URL,
		"source":             doc.Source.Name,
		"source_url":         doc.SourceURL,
		"image_url":          doc.ImageURL,
		"tags":               []string(doc.Tags),
		"num_highlights":     len(doc.Highlights),
		"last_highlighted_at": formatTime(doc.LastHighlightedAt),
		"created_at":         doc.CreatedAt.Format("Jan 2, 2006"),
		"updated_at":         doc.UpdatedAt.Format("Jan 2, 2006"),
		"highlights":         highlights,
		"metadata":           map[string]interface{}(doc.Metadata),
	}

	// Also expose site_name from metadata if available
	if sn, ok := doc.Metadata["site_name"]; ok {
		ctx["site_name"] = sn
	}

	content, err := tpl.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	content = strings.TrimSpace(content)
	checksum := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))

	return &ExportedDocument{
		ID:             doc.ID,
		Type:           doc.Type,
		Title:          doc.Title,
		Author:         doc.Author,
		Content:        content,
		UpdatedAt:      doc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		HighlightCount: len(doc.Highlights),
		Checksum:       checksum,
	}, nil
}

// GetDefaultTemplate returns the built-in default template for a document type.
func GetDefaultTemplate(docType string) models.Template {
	switch docType {
	case "article":
		return models.Template{
			ID:           "default-article",
			Name:         "Default Article",
			Type:         "article",
			PageTemplate: DefaultArticlePageTemplate,
			IsDefault:    true,
		}
	default:
		return models.Template{
			ID:           "default-book",
			Name:         "Default Book",
			Type:         "book",
			PageTemplate: DefaultBookPageTemplate,
			IsDefault:    true,
		}
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("Jan 2, 2006")
}
