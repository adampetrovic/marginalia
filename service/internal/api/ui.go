package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/render"
	"github.com/adampetrovic/marginalia/service/internal/ui"
)

// registerUIRoutes adds the web UI routes to the router.
func (s *Server) registerUIRoutes(r chi.Router) {
	r.Route("/ui", func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Get("/", s.uiDashboard)
		r.Post("/sync", s.uiSyncAll)

		r.Get("/templates", s.uiTemplateList)
		r.Get("/templates/new", s.uiTemplateNew)
		r.Post("/templates/new", s.uiTemplateCreate)
		r.Get("/templates/{id}", s.uiTemplateEdit)
		r.Post("/templates/{id}", s.uiTemplateUpdate)
		r.Post("/templates/validate", s.uiTemplateValidate)
		r.Post("/templates/preview", s.uiTemplatePreview)

		r.Get("/documents/{id}", s.uiDocumentDetail)

		r.Get("/history", s.uiSyncHistory)
	})
}

// --- Dashboard ---

func (s *Server) uiDashboard(w http.ResponseWriter, r *http.Request) {
	var bookCount, articleCount, hlCount int64
	s.db.Model(&models.Document{}).Where("type = ?", "book").Count(&bookCount)
	s.db.Model(&models.Document{}).Where("type = ?", "article").Count(&articleCount)
	s.db.Model(&models.Highlight{}).Count(&hlCount)

	var sources []models.Source
	s.db.Find(&sources)

	// Tab filter
	tab := r.URL.Query().Get("tab") // "all", "book", "article"
	if tab == "" {
		tab = "all"
	}

	// Title search
	titleQuery := r.URL.Query().Get("q")

	// Build document query
	docQ := s.db.Preload("Highlights").Order("updated_at DESC")
	if tab != "all" {
		docQ = docQ.Where("type = ?", tab)
	}
	if titleQuery != "" {
		docQ = docQ.Where("title LIKE ? OR author LIKE ?", "%"+titleQuery+"%", "%"+titleQuery+"%")
	}
	var docs []models.Document
	docQ.Limit(50).Find(&docs)

	// Highlight search
	highlightQuery := r.URL.Query().Get("hl")
	var matchedHighlights []models.Highlight
	if highlightQuery != "" {
		s.db.Preload("Document").
			Where("text LIKE ? OR note LIKE ?", "%"+highlightQuery+"%", "%"+highlightQuery+"%").
			Order("created_at DESC").Limit(50).
			Find(&matchedHighlights)
	}

	// Last sync time
	var lastLog models.SyncLog
	s.db.Where("status = ?", "completed").Order("completed_at DESC").First(&lastLog)

	renderPage(w, "dashboard.html", map[string]interface{}{
		"Nav":               "dashboard",
		"Title":             "Highlights",
		"BookCount":         bookCount,
		"ArticleCount":      articleCount,
		"HighlightCount":    hlCount,
		"Sources":           sources,
		"Documents":         docs,
		"LastSync":          lastLog,
		"Tab":               tab,
		"Query":             titleQuery,
		"HighlightQuery":    highlightQuery,
		"MatchedHighlights": matchedHighlights,
	})
}

// --- Sync ---

func (s *Server) uiSyncAll(w http.ResponseWriter, r *http.Request) {
	results := map[string]interface{}{}

	if s.cfg.IsReadeckConfigured() {
		res, err := s.syncReadeck()
		if err != nil {
			s.logger.Error("readeck sync failed", "error", err)
			renderPartial(w, "sync-error", err.Error())
			return
		}
		results["readeck"] = map[string]interface{}{
			"Documents":  res.DocumentsSynced,
			"Highlights": res.HighlightsSynced,
		}
	}

	renderPartial(w, "sync-success", results)
}

// --- Document Detail ---

func (s *Server) uiDocumentDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var doc models.Document
	if err := s.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_sort_key ASC, created_at ASC")
	}).Preload("Source").First(&doc, "id = ?", id).Error; err != nil {
		http.Redirect(w, r, "/ui/", http.StatusSeeOther)
		return
	}

	renderPage(w, "document.html", map[string]interface{}{
		"Nav":      "dashboard",
		"Title":    doc.Title,
		"Document": doc,
	})
}

// --- Templates ---

func (s *Server) uiTemplateList(w http.ResponseWriter, r *http.Request) {
	var templates []models.Template
	s.db.Order("type ASC, name ASC").Find(&templates)

	renderPage(w, "templates.html", map[string]interface{}{
		"Nav":                    "templates",
		"Title":                  "Templates",
		"Templates":              templates,
		"DefaultBookTemplate":    render.DefaultBookPageTemplate,
		"DefaultArticleTemplate": render.DefaultArticlePageTemplate,
	})
}

func (s *Server) uiTemplateNew(w http.ResponseWriter, r *http.Request) {
	renderPage(w, "template_edit.html", map[string]interface{}{
		"Nav":      "templates",
		"Title":    "New Template",
		"IsNew":    true,
		"Template": models.Template{Type: "book", PageTemplate: render.DefaultBookPageTemplate},
	})
}

func (s *Server) uiTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, "template_edit.html", map[string]interface{}{
			"Nav": "templates", "Title": "New Template", "IsNew": true,
			"Template": models.Template{}, "Error": "Invalid form data",
		})
		return
	}

	tmpl := models.Template{
		ID:           fmt.Sprintf("tmpl-%d", time.Now().UnixNano()),
		Name:         r.FormValue("name"),
		Type:         r.FormValue("type"),
		PageTemplate: r.FormValue("page_template"),
		IsDefault:    r.FormValue("is_default") == "on",
	}

	// Validate template syntax
	if err := render.ValidateTemplate(tmpl.PageTemplate); err != nil {
		renderPage(w, "template_edit.html", map[string]interface{}{
			"Nav": "templates", "Title": "New Template", "IsNew": true,
			"Template": tmpl, "Error": "Template syntax error: " + err.Error(),
		})
		return
	}

	if err := s.db.Create(&tmpl).Error; err != nil {
		renderPage(w, "template_edit.html", map[string]interface{}{
			"Nav": "templates", "Title": "New Template", "IsNew": true,
			"Template": tmpl, "Error": err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/ui/templates", http.StatusSeeOther)
}

func (s *Server) uiTemplateEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var tmpl models.Template
	if err := s.db.First(&tmpl, "id = ?", id).Error; err != nil {
		http.Redirect(w, r, "/ui/templates", http.StatusSeeOther)
		return
	}

	renderPage(w, "template_edit.html", map[string]interface{}{
		"Nav":      "templates",
		"Title":    "Edit: " + tmpl.Name,
		"IsNew":    false,
		"Template": tmpl,
		"Success":  r.URL.Query().Get("saved"),
	})
}

func (s *Server) uiTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var tmpl models.Template
	if err := s.db.First(&tmpl, "id = ?", id).Error; err != nil {
		http.Redirect(w, r, "/ui/templates", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderPage(w, "template_edit.html", map[string]interface{}{
			"Nav": "templates", "Title": "Edit: " + tmpl.Name,
			"Template": tmpl, "Error": "Invalid form data",
		})
		return
	}

	newPageTemplate := r.FormValue("page_template")

	// Validate template syntax
	if err := render.ValidateTemplate(newPageTemplate); err != nil {
		tmpl.PageTemplate = newPageTemplate
		renderPage(w, "template_edit.html", map[string]interface{}{
			"Nav": "templates", "Title": "Edit: " + tmpl.Name,
			"Template": tmpl, "Error": "Template syntax error: " + err.Error(),
		})
		return
	}

	s.db.Model(&tmpl).Updates(map[string]interface{}{
		"name":          r.FormValue("name"),
		"type":          r.FormValue("type"),
		"page_template": newPageTemplate,
		"is_default":    r.FormValue("is_default") == "on",
	})

	http.Redirect(w, r, fmt.Sprintf("/ui/templates/%s?saved=Template+saved", id), http.StatusSeeOther)
}

func (s *Server) uiTemplateValidate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPartial(w, "validation-error", "Invalid request")
		return
	}

	if err := render.ValidateTemplate(r.FormValue("page_template")); err != nil {
		renderPartial(w, "validation-error", err.Error())
		return
	}

	renderPartial(w, "validation-ok", nil)
}

func (s *Server) uiTemplatePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPartial(w, "preview-error", "Invalid request")
		return
	}

	pageTemplate := r.FormValue("page_template")
	docType := r.FormValue("type")

	sampleDoc := render.SampleDocument(docType)
	tmpl := models.Template{PageTemplate: pageTemplate}

	exported, err := s.renderer.RenderDocument(sampleDoc, tmpl)
	if err != nil {
		renderPartial(w, "preview-error", err.Error())
		return
	}

	renderPartial(w, "preview-result", exported.Content)
}

// --- Sync History ---

func (s *Server) uiSyncHistory(w http.ResponseWriter, r *http.Request) {
	var logs []models.SyncLog
	s.db.Order("started_at DESC").Limit(50).Find(&logs)

	renderPage(w, "history.html", map[string]interface{}{
		"Nav":   "history",
		"Title": "Sync History",
		"Logs":  logs,
	})
}

// --- Render helpers ---

func renderPage(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.RenderPage(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.RenderPartial(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
