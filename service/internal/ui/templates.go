// Package ui provides an embedded web UI for Marginalia.
// Uses html/template for server-side rendering and htmx for interactivity.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

//go:embed templates/*.html
var templateFS embed.FS

var funcs = template.FuncMap{
	"timeAgo": func(t *time.Time) string {
		if t == nil {
			return "never"
		}
		d := time.Since(*t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			m := int(d.Minutes())
			if m == 1 {
				return "1 minute ago"
			}
			return fmt.Sprintf("%d minutes ago", m)
		case d < 24*time.Hour:
			h := int(d.Hours())
			if h == 1 {
				return "1 hour ago"
			}
			return fmt.Sprintf("%d hours ago", h)
		default:
			days := int(d.Hours() / 24)
			if days == 1 {
				return "1 day ago"
			}
			return fmt.Sprintf("%d days ago", days)
		}
	},
	"fmtTime": func(t time.Time) string {
		return t.Format("2 Jan 2006 15:04")
	},
	"fmtTimePtr": func(t *time.Time) string {
		if t == nil {
			return "—"
		}
		return t.Format("2 Jan 2006 15:04")
	},
	"isHeading": func(tags models.JSONStringArray) bool {
		for _, t := range tags {
			if strings.HasPrefix(t, "h") && len(t) == 2 && t[1] >= '1' && t[1] <= '5' {
				return true
			}
		}
		return false
	},
	"headingTag": func(tags models.JSONStringArray) string {
		for _, t := range tags {
			if strings.HasPrefix(t, "h") && len(t) == 2 && t[1] >= '1' && t[1] <= '5' {
				return t
			}
		}
		return ""
	},
}

// pages holds a separately parsed template for each page.
// Each page template is combined with layout.html and partials.html
// so that {{define "content"}} in one page doesn't clobber another.
var pages map[string]*template.Template

// partials holds templates for htmx fragment responses (no layout).
var partials *template.Template

func init() {
	shared := []string{"templates/layout.html"}
	pageFiles := []string{
		"templates/dashboard.html",
		"templates/document.html",
		"templates/templates.html",
		"templates/template_edit.html",
		"templates/history.html",
	}

	pages = make(map[string]*template.Template, len(pageFiles))
	for _, pf := range pageFiles {
		files := []string{shared[0], pf}
		t := template.Must(
			template.New("").Funcs(funcs).ParseFS(templateFS, files...),
		)
		pages[pf] = t
	}

	// Partials are standalone fragments (no layout wrapper).
	partials = template.Must(
		template.New("").Funcs(funcs).ParseFS(templateFS, "templates/partials.html"),
	)
}

// RenderPage executes a page template (wrapped in layout) into the writer.
// name is the filename inside templates/, e.g. "dashboard.html".
func RenderPage(w io.Writer, name string, data interface{}) error {
	key := "templates/" + name
	t, ok := pages[key]
	if !ok {
		return fmt.Errorf("unknown page template: %s", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

// RenderPartial executes a named partial template (no layout).
func RenderPartial(w io.Writer, name string, data interface{}) error {
	return partials.ExecuteTemplate(w, name, data)
}
