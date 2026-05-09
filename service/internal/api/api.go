package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"

	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/render"
	"github.com/adampetrovic/marginalia/service/internal/sync/koreader"
	"github.com/adampetrovic/marginalia/service/internal/sync/readeck"
)

// Server is the HTTP API server.
type Server struct {
	cfg      *config.Config
	db       *gorm.DB
	renderer *render.Renderer
	router   chi.Router
	logger   *slog.Logger
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, db *gorm.DB, logger *slog.Logger) *Server {
	s := &Server{
		cfg:      cfg,
		db:       db,
		renderer: render.New(),
		logger:   logger,
	}
	s.router = s.buildRouter()
	return s
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check (unauthenticated)
	r.Get("/healthz", s.handleHealthz)

	// Authenticated API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Route("/v1", func(r chi.Router) {
			// Sources
			r.Get("/sources", s.handleListSources)

			// Sync
			r.Post("/sync", s.handleSyncAll)
			r.Post("/sync/{source}", s.handleSyncSource)
			r.Get("/sync/status", s.handleSyncStatus)

			// Documents & Highlights
			r.Get("/documents", s.handleListDocuments)
			r.Get("/documents/{id}", s.handleGetDocument)
			r.Get("/highlights", s.handleListHighlights)

			// Daily review
			r.Get("/review", s.handleGetReview)
			r.Post("/review/{id}", s.handleReviewAction)

			// Templates
			r.Get("/templates", s.handleListTemplates)
			r.Get("/templates/{id}", s.handleGetTemplate)
			r.Post("/templates", s.handleCreateTemplate)
			r.Put("/templates/{id}", s.handleUpdateTemplate)
			r.Post("/templates/preview", s.handlePreviewTemplate)

			// Export
			r.Get("/export", s.handleExport)
			r.Get("/export/documents/{id}", s.handleExportDocument)
		})

		// Readwise-compatible endpoints (for KOReader) — kept at /api/v2 for compatibility
		r.Post("/v2/highlights", s.handleReadwiseHighlights)
		r.Get("/v2/auth", s.handleReadwiseAuth)
	})

	// Web UI (authenticated, served at root)
	s.registerUIRoutes(r)

	return r
}

// authMiddleware validates the bearer token.
// Supports: Authorization header (Bearer/Token), query param (?token=), or cookie.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""

		// 1. Authorization header (API clients, KOReader)
		if auth := r.Header.Get("Authorization"); auth != "" {
			token = auth
			for _, prefix := range []string{"Bearer ", "Token "} {
				if len(token) > len(prefix) && token[:len(prefix)] == prefix {
					token = token[len(prefix):]
					break
				}
			}
		}

		// 2. Cookie (web UI sessions)
		if token == "" {
			if c, err := r.Cookie("marginalia_token"); err == nil {
				token = c.Value
			}
		}

		// 3. Query param (web UI initial login, sets cookie)
		if token == "" {
			token = r.URL.Query().Get("token")
			if token == s.cfg.APIToken {
				http.SetCookie(w, &http.Cookie{
					Name:     "marginalia_token",
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   86400 * 30, // 30 days
				})
				// Redirect to strip token from URL
				clean := *r.URL
				q := clean.Query()
				q.Del("token")
				clean.RawQuery = q.Encode()
				http.Redirect(w, r, clean.String(), http.StatusFound)
				return
			}
		}

		if token == "" || token != s.cfg.APIToken {
			// For UI routes, show a simple message instead of JSON
			isAPIRoute := len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api"
			if !isAPIRoute {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `<html><body style="font-family: sans-serif; padding: 2rem; background: #0f1117; color: #e4e4e7;">
					<h2>Marginalia</h2><p>Append <code>?token=YOUR_API_TOKEN</code> to the URL to log in.</p></body></html>`)
				return
			}
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Health ---

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Sources ---

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	var sources []models.Source
	s.db.Find(&sources)
	writeJSON(w, http.StatusOK, sources)
}

// --- Sync ---

func (s *Server) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	results := map[string]interface{}{}

	if s.cfg.IsReadeckConfigured() {
		res, err := s.syncReadeck()
		if err != nil {
			s.logger.Error("readeck sync failed", "error", err)
			results["readeck"] = map[string]string{"status": "failed", "error": err.Error()}
		} else {
			results["readeck"] = map[string]interface{}{
				"status":     "completed",
				"documents":  res.DocumentsSynced,
				"highlights": res.HighlightsSynced,
			}
		}
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleSyncSource(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")

	switch source {
	case "readeck":
		if !s.cfg.IsReadeckConfigured() {
			writeError(w, http.StatusBadRequest, "readeck not configured")
			return
		}
		res, err := s.syncReadeck()
		if err != nil {
			s.logger.Error("readeck sync failed", "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "completed",
			"documents":  res.DocumentsSynced,
			"highlights": res.HighlightsSynced,
		})
	default:
		writeError(w, http.StatusBadRequest, "unknown source: "+source)
	}
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	var logs []models.SyncLog
	s.db.Order("started_at DESC").Limit(10).Find(&logs)
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) syncReadeck() (*readeck.SyncResult, error) {
	src, err := readeck.EnsureSource(s.db)
	if err != nil {
		return nil, err
	}

	// Log sync start
	syncLog := models.SyncLog{
		SourceID:  src.ID,
		Status:    "started",
		StartedAt: time.Now(),
	}
	s.db.Create(&syncLog)

	client := readeck.NewClient(s.cfg.ReadeckURL, s.cfg.ReadeckToken)
	syncer := readeck.NewSyncer(client, s.db)
	result, err := syncer.Sync(src.ID)

	now := time.Now()
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
	} else {
		syncLog.Status = "completed"
		syncLog.DocumentsSynced = result.DocumentsSynced
		syncLog.HighlightsSynced = result.HighlightsSynced
	}
	syncLog.CompletedAt = &now
	s.db.Save(&syncLog)

	return result, err
}

// --- Documents ---

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	var docs []models.Document
	q := s.db.Preload("Source")

	if docType := r.URL.Query().Get("type"); docType != "" {
		q = q.Where("type = ?", docType)
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q = q.Where("updated_at > ?", t)
		}
	}

	q.Order("updated_at DESC").Find(&docs)
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var doc models.Document
	if err := s.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_sort_key ASC, created_at ASC")
	}).Preload("Source").First(&doc, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// --- Highlights ---

func (s *Server) handleListHighlights(w http.ResponseWriter, r *http.Request) {
	var highlights []models.Highlight
	q := s.db.Model(&models.Highlight{})

	if docID := r.URL.Query().Get("document_id"); docID != "" {
		q = q.Where("document_id = ?", docID)
	}

	q.Order("created_at DESC").Limit(100).Find(&highlights)
	writeJSON(w, http.StatusOK, highlights)
}

// --- Templates ---

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	var templates []models.Template
	s.db.Find(&templates)
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var tmpl models.Template
	if err := s.db.First(&tmpl, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var tmpl models.Template
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if tmpl.ID == "" {
		tmpl.ID = fmt.Sprintf("tmpl-%d", time.Now().UnixNano())
	}
	if err := s.db.Create(&tmpl).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tmpl)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var existing models.Template
	if err := s.db.First(&existing, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	var update models.Template
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.db.Model(&existing).Updates(map[string]interface{}{
		"name":               update.Name,
		"page_template":      update.PageTemplate,
		"highlight_template": update.HighlightTemplate,
		"is_default":         update.IsDefault,
	})
	s.db.First(&existing, "id = ?", id)
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handlePreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PageTemplate string `json:"page_template"`
		Type         string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sampleDoc := render.SampleDocument(req.Type)
	tmpl := models.Template{PageTemplate: req.PageTemplate}

	exported, err := s.renderer.RenderDocument(sampleDoc, tmpl)
	if err != nil {
		writeError(w, http.StatusBadRequest, "template error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exported)
}

// --- Export ---

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var docs []models.Document
	q := s.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_sort_key ASC, created_at ASC")
	}).Preload("Source")

	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q = q.Where("updated_at > ?", t)
		}
	}

	q.Order("updated_at DESC").Find(&docs)

	exported := make([]*render.ExportedDocument, 0, len(docs))
	for _, doc := range docs {
		tmpl := s.getTemplateForDoc(doc)
		exp, err := s.renderer.RenderDocument(doc, tmpl)
		if err != nil {
			s.logger.Error("failed to render document", "doc_id", doc.ID, "error", err)
			continue
		}
		exported = append(exported, exp)
	}

	writeJSON(w, http.StatusOK, exported)
}

func (s *Server) handleExportDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var doc models.Document
	if err := s.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_sort_key ASC, created_at ASC")
	}).Preload("Source").First(&doc, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	tmpl := s.getTemplateForDoc(doc)
	exported, err := s.renderer.RenderDocument(doc, tmpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "render error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, exported)
}

func (s *Server) getTemplateForDoc(doc models.Document) models.Template {
	var tmpl models.Template
	err := s.db.Where("type = ? AND is_default = ?", doc.Type, true).First(&tmpl).Error
	if err != nil {
		return render.GetDefaultTemplate(doc.Type)
	}
	return tmpl
}

// --- Readwise-compatible endpoints ---

func (s *Server) handleReadwiseHighlights(w http.ResponseWriter, r *http.Request) {
	var req koreader.ReadwiseHighlightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	src, err := koreader.EnsureSource(s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := koreader.Ingest(s.db, src.ID, req)
	if err != nil {
		s.logger.Error("koreader ingest failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":  result.DocumentsSynced,
		"highlights": result.HighlightsSynced,
	})
}

func (s *Server) handleReadwiseAuth(w http.ResponseWriter, r *http.Request) {
	// KOReader checks this endpoint to validate the token
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
