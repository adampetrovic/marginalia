// Package ui provides an embedded web UI for Marginalia.
// Uses html/template for server-side rendering and htmx for interactivity.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"time"
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
}

var templates *template.Template

func init() {
	templates = template.Must(
		template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html"),
	)
}

// Render executes a named template into the writer.
func Render(w io.Writer, name string, data interface{}) error {
	return templates.ExecuteTemplate(w, name, data)
}
