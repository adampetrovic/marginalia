package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/sync/readeck"
)

// --- Dashboard stats ---

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	uid := s.currentUser(r).ID

	var books, articles, highlights, due int64
	s.db.Model(&models.Document{}).Where("user_id = ? AND type = ?", uid, "book").Count(&books)
	s.db.Model(&models.Document{}).Where("user_id = ? AND type = ?", uid, "article").Count(&articles)
	s.db.Model(&models.Highlight{}).Where("user_id = ?", uid).Count(&highlights)

	stats, err := s.reviewStats(uid, time.Now())
	if err == nil {
		due = stats.Due + stats.New
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"books":       books,
		"articles":    articles,
		"highlights":  highlights,
		"due_reviews": due,
	})
}

// --- Readeck integration (per-user) ---

// readeckConfig returns the user's stored Readeck URL and token, and whether
// both are present.
func (s *Server) readeckConfig(userID string) (url, token string, ok bool) {
	var src models.Source
	if err := s.db.First(&src, "id = ?", "readeck-"+userID).Error; err != nil {
		return "", "", false
	}
	url, _ = src.Config["url"].(string)
	token, _ = src.Config["token"].(string)
	return url, token, url != "" && token != ""
}

func (s *Server) handleGetReadeckIntegration(w http.ResponseWriter, r *http.Request) {
	url, token, configured := s.readeckConfig(s.currentUser(r).ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"url":        url,
		"token":      token,
		"configured": configured,
	})
}

func (s *Server) handleUpdateReadeckIntegration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	src, err := readeck.EnsureSource(s.db, s.currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	src.Config = models.JSONMap{"url": req.URL, "token": req.Token}
	if err := s.db.Model(&src).Update("config", src.Config).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configured": req.URL != "" && req.Token != "",
	})
}
