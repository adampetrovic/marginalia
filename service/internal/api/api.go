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

	"github.com/adampetrovic/marginalia/service/internal/auth"
	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/render"
	"github.com/adampetrovic/marginalia/service/internal/sync/koreader"
	"github.com/adampetrovic/marginalia/service/internal/sync/readeck"
)

// Server is the HTTP API server.
type Server struct {
	cfg           *config.Config
	db            *gorm.DB
	renderer      *render.Renderer
	router        chi.Router
	logger        *slog.Logger
	sessionSecret []byte
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, db *gorm.DB, logger *slog.Logger) *Server {
	secret := cfg.SessionSecret
	if secret == "" {
		generated, err := auth.RandomSecret()
		if err != nil {
			panic("generating session secret: " + err.Error())
		}
		secret = generated
		logger.Warn("MARGINALIA_SESSION_SECRET not set; using a random secret (sessions reset on restart)")
	}

	s := &Server{
		cfg:           cfg,
		db:            db,
		renderer:      render.New(),
		logger:        logger,
		sessionSecret: []byte(secret),
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

	r.Route("/api", func(r chi.Router) {
		// Public auth endpoints (no session required).
		r.Route("/v1/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/logout", s.handleLogout)
			r.With(s.authMiddleware).Get("/me", s.handleMe)
		})

		// Authenticated API routes.
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			r.Route("/v1", func(r chi.Router) {
				// Current-user dashboard stats
				r.Get("/stats", s.handleStats)

				// API tokens
				r.Get("/tokens", s.handleListTokens)
				r.Post("/tokens", s.handleCreateToken)
				r.Delete("/tokens/{id}", s.handleDeleteToken)

				// Integrations
				r.Get("/integrations/readeck", s.handleGetReadeckIntegration)
				r.Put("/integrations/readeck", s.handleUpdateReadeckIntegration)

				// Sources
				r.Get("/sources", s.handleListSources)

				// Sync
				r.Post("/sync", s.handleSyncAll)
				r.Post("/sync/{source}", s.handleSyncSource)
				r.Get("/sync/status", s.handleSyncStatus)

				// Documents & Highlights
				r.Get("/documents", s.handleListDocuments)
				r.Get("/documents/{id}", s.handleGetDocument)
				r.Put("/documents/{id}", s.handleUpdateDocument)
				r.Delete("/documents/{id}", s.handleDeleteDocument)
				r.Get("/highlights", s.handleListHighlights)
				r.Get("/highlights/{id}", s.handleGetHighlight)
				r.Put("/highlights/{id}", s.handleUpdateHighlight)
				r.Delete("/highlights/{id}", s.handleDeleteHighlight)
				r.Post("/highlights/{id}/favorite", s.handleFavoriteHighlight)

				// Daily review
				r.Get("/review", s.handleGetReview)
				r.Post("/review/{id}", s.handleReviewAction)

				// Templates
				r.Get("/templates", s.handleListTemplates)
				r.Get("/templates/{id}", s.handleGetTemplate)
				r.Post("/templates", s.handleCreateTemplate)
				r.Put("/templates/{id}", s.handleUpdateTemplate)
				r.Delete("/templates/{id}", s.handleDeleteTemplate)
				r.Post("/templates/preview", s.handlePreviewTemplate)

				// Export
				r.Get("/export", s.handleExport)
				r.Get("/export/documents/{id}", s.handleExportDocument)
			})

			// Readwise-compatible endpoints for KOReader, Readest, and other
			// Readwise clients — kept at /api/v2. Clients build the path
			// differently: KOReader posts to ".../api/v2/highlights" (no trailing
			// slash), while Readest uses a custom base URL of ".../api/v2" and
			// appends "/highlights/" and "/auth/" with a trailing slash. Register
			// both forms so either client works.
			r.Post("/v2/highlights", s.handleReadwiseHighlights)
			r.Post("/v2/highlights/", s.handleReadwiseHighlights)
			r.Get("/v2/auth", s.handleReadwiseAuth)
			r.Get("/v2/auth/", s.handleReadwiseAuth)
		})
	})

	// Single-page app (built React frontend), served at root with history fallback.
	s.registerWebRoutes(r)

	return r
}

// --- Health ---

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Sources ---

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	var sources []models.Source
	s.scoped(r).Find(&sources)
	if sources == nil {
		sources = []models.Source{}
	}
	writeJSON(w, http.StatusOK, sources)
}

// --- Sync ---

func (s *Server) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	results := map[string]interface{}{}

	if url, token, ok := s.readeckConfig(user.ID); ok {
		res, err := s.syncReadeck(user, url, token)
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
	user := s.currentUser(r)
	source := chi.URLParam(r, "source")

	switch source {
	case "readeck":
		url, token, ok := s.readeckConfig(user.ID)
		if !ok {
			writeError(w, http.StatusBadRequest, "readeck not configured")
			return
		}
		res, err := s.syncReadeck(user, url, token)
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
	s.scoped(r).Order("started_at DESC").Limit(10).Find(&logs)
	if logs == nil {
		logs = []models.SyncLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) syncReadeck(user *models.User, url, token string) (*readeck.SyncResult, error) {
	src, err := readeck.EnsureSource(s.db, user.ID)
	if err != nil {
		return nil, err
	}

	// Log sync start
	syncLog := models.SyncLog{
		UserID:    user.ID,
		SourceID:  src.ID,
		Status:    "started",
		StartedAt: time.Now(),
	}
	s.db.Create(&syncLog)

	client := readeck.NewClient(url, token)
	syncer := readeck.NewSyncer(client, s.db)
	result, err := syncer.Sync(src)

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
	q := s.scoped(r).Preload("Highlights").Preload("Source")

	if docType := r.URL.Query().Get("type"); docType != "" {
		q = q.Where("type = ?", docType)
	}
	if query := r.URL.Query().Get("q"); query != "" {
		like := "%" + query + "%"
		q = q.Where("title LIKE ? OR author LIKE ?", like, like)
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q = q.Where("updated_at > ?", t)
		}
	}

	q.Order("updated_at DESC").Find(&docs)
	if docs == nil {
		docs = []models.Document{}
	}
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var doc models.Document
	if err := s.scoped(r).Preload("Highlights", func(db *gorm.DB) *gorm.DB {
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
	q := s.scoped(r).Model(&models.Highlight{}).Preload("Document")

	if docID := r.URL.Query().Get("document_id"); docID != "" {
		q = q.Where("document_id = ?", docID)
	}
	if query := r.URL.Query().Get("q"); query != "" {
		like := "%" + query + "%"
		q = q.Where("text LIKE ? OR note LIKE ?", like, like)
	}

	q.Order("created_at DESC").Limit(100).Find(&highlights)
	if highlights == nil {
		highlights = []models.Highlight{}
	}
	writeJSON(w, http.StatusOK, highlights)
}

// --- Templates ---

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	var templates []models.Template
	s.scoped(r).Order("type ASC, name ASC").Find(&templates)
	if templates == nil {
		templates = []models.Template{}
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var tmpl models.Template
	if err := s.scoped(r).First(&tmpl, "id = ?", id).Error; err != nil {
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
	tmpl.UserID = s.currentUser(r).ID
	if err := s.db.Create(&tmpl).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tmpl)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var existing models.Template
	if err := s.scoped(r).First(&existing, "id = ?", id).Error; err != nil {
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
		"type":               update.Type,
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
	q := s.scoped(r).Preload("Highlights", func(db *gorm.DB) *gorm.DB {
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
	if err := s.scoped(r).Preload("Highlights", func(db *gorm.DB) *gorm.DB {
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
	err := s.db.Where("user_id = ? AND type = ? AND is_default = ?", doc.UserID, doc.Type, true).First(&tmpl).Error
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

	src, err := koreader.EnsureSource(s.db, s.currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := koreader.Ingest(s.db, src, req)
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
