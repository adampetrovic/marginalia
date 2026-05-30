package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

// --- Highlights CRUD ---

func (s *Server) handleGetHighlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var hl models.Highlight
	if err := s.scoped(r).Preload("Document").Preload("ReviewState").First(&hl, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "highlight not found")
		return
	}
	writeJSON(w, http.StatusOK, hl)
}

// handleUpdateHighlight applies a partial update to a highlight. Editing any of the
// user-owned content fields (text, note, color, tags) marks the highlight as
// UserEdited so a future re-sync will not overwrite the change.
func (s *Server) handleUpdateHighlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var hl models.Highlight
	if err := s.scoped(r).First(&hl, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "highlight not found")
		return
	}

	var req struct {
		Text     *string                 `json:"text"`
		Note     *string                 `json:"note"`
		Color    *string                 `json:"color"`
		Tags     *models.JSONStringArray `json:"tags"`
		Favorite *bool                   `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := map[string]interface{}{}
	contentChanged := false
	if req.Text != nil {
		updates["text"] = *req.Text
		contentChanged = true
	}
	if req.Note != nil {
		updates["note"] = *req.Note
		contentChanged = true
	}
	if req.Color != nil {
		updates["color"] = *req.Color
		contentChanged = true
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
		contentChanged = true
	}
	if req.Favorite != nil {
		updates["favorite"] = *req.Favorite
	}
	if contentChanged {
		updates["user_edited"] = true
	}

	if len(updates) > 0 {
		if err := s.db.Model(&hl).Updates(updates).Error; err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.db.First(&hl, "id = ?", id)
	writeJSON(w, http.StatusOK, hl)
}

func (s *Server) handleDeleteHighlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res := s.scoped(r).Where("id = ?", id).Delete(&models.Highlight{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "highlight not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFavoriteHighlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var hl models.Highlight
	if err := s.scoped(r).First(&hl, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "highlight not found")
		return
	}
	if err := s.db.Model(&hl).Update("favorite", !hl.Favorite).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.db.First(&hl, "id = ?", id)
	writeJSON(w, http.StatusOK, hl)
}

// --- Documents CRUD ---

func (s *Server) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var doc models.Document
	if err := s.scoped(r).First(&doc, "id = ?", id).Error; err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	var req struct {
		Title    *string                 `json:"title"`
		Author   *string                 `json:"author"`
		Tags     *models.JSONStringArray `json:"tags"`
		Favorite *bool                   `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Author != nil {
		updates["author"] = *req.Author
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.Favorite != nil {
		updates["favorite"] = *req.Favorite
	}

	if len(updates) > 0 {
		if err := s.db.Model(&doc).Updates(updates).Error; err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.db.First(&doc, "id = ?", id)
	writeJSON(w, http.StatusOK, doc)
}

// handleDeleteDocument soft-deletes a document and its highlights.
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res := s.scoped(r).Where("id = ?", id).Delete(&models.Document{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	// Cascade: soft-delete the document's highlights so they don't linger in
	// review queues or search.
	s.scoped(r).Where("document_id = ?", id).Delete(&models.Highlight{})
	w.WriteHeader(http.StatusNoContent)
}

// --- Templates ---

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res := s.scoped(r).Where("id = ?", id).Delete(&models.Template{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
