package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

const defaultEaseFactor = 2.5

type reviewQueueStats struct {
	Due           int64 `json:"due"`
	New           int64 `json:"new"`
	ReviewedToday int64 `json:"reviewed_today"`
}

type reviewActionRequest struct {
	Rating string `json:"rating"`
}

// --- Review API ---

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	card, err := s.reviewCard(s.currentUser(r).ID, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleReviewAction(w http.ResponseWriter, r *http.Request) {
	userID := s.currentUser(r).ID
	highlightID := chi.URLParam(r, "id")
	var req reviewActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := s.applyReviewAction(userID, highlightID, req.Rating, time.Now()); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	// Return the next due card so the client can advance immediately.
	card, err := s.reviewCard(userID, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, card)
}

// --- Review scheduling ---

// reviewCard assembles the next-due-card response for a user: either the next
// highlight with its document and review state, or {"done": true} when the
// queue is empty. Either way it carries the current queue stats.
func (s *Server) reviewCard(userID string, now time.Time) (map[string]interface{}, error) {
	highlight, state, err := s.nextDueReview(userID, now)
	if err != nil {
		return nil, err
	}
	stats, err := s.reviewStats(userID, now)
	if err != nil {
		return nil, err
	}

	resp := map[string]interface{}{"stats": stats}
	if highlight == nil {
		resp["done"] = true
		return resp, nil
	}
	resp["done"] = false
	resp["highlight"] = highlight
	resp["state"] = state
	resp["document"] = highlight.Document
	return resp, nil
}

func (s *Server) nextDueReview(userID string, now time.Time) (*models.Highlight, *models.ReviewState, error) {
	var highlight models.Highlight
	err := s.dueReviewQuery(userID, now).
		Preload("Document").
		Preload("ReviewState").
		Order("COALESCE(review_states.due_at, highlights.created_at) ASC").
		Order("highlights.created_at ASC").
		First(&highlight).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	return &highlight, highlight.ReviewState, nil
}

func (s *Server) reviewStats(userID string, now time.Time) (reviewQueueStats, error) {
	var stats reviewQueueStats
	if err := s.dueReviewQuery(userID, now).Count(&stats.Due).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Highlight{}).
		Joins("LEFT JOIN review_states ON review_states.highlight_id = highlights.id").
		Where("highlights.user_id = ? AND review_states.highlight_id IS NULL", userID).
		Count(&stats.New).Error; err != nil {
		return stats, err
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if err := s.db.Model(&models.ReviewState{}).
		Where("user_id = ? AND last_reviewed_at >= ?", userID, startOfDay).
		Count(&stats.ReviewedToday).Error; err != nil {
		return stats, err
	}

	return stats, nil
}

func (s *Server) dueReviewQuery(userID string, now time.Time) *gorm.DB {
	return s.db.Model(&models.Highlight{}).
		Joins("LEFT JOIN review_states ON review_states.highlight_id = highlights.id").
		Where("highlights.user_id = ?", userID).
		Where("review_states.highlight_id IS NULL OR (review_states.suspended = ? AND review_states.due_at <= ?)", false, now)
}

func (s *Server) applyReviewAction(userID, highlightID, rating string, now time.Time) (*models.ReviewState, error) {
	if rating == "" {
		return nil, errors.New("rating is required")
	}

	var highlight models.Highlight
	if err := s.db.First(&highlight, "id = ? AND user_id = ?", highlightID, userID).Error; err != nil {
		return nil, err
	}

	var state models.ReviewState
	err := s.db.First(&state, "highlight_id = ?", highlightID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state = models.ReviewState{HighlightID: highlightID, UserID: userID, EaseFactor: defaultEaseFactor}
	} else if err != nil {
		return nil, err
	}

	if state.EaseFactor == 0 {
		state.EaseFactor = defaultEaseFactor
	}

	if err := scheduleReview(&state, rating, now); err != nil {
		return nil, err
	}
	if err := s.db.Save(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func scheduleReview(state *models.ReviewState, rating string, now time.Time) error {
	state.LastReviewedAt = &now
	state.LastRating = rating

	switch rating {
	case "again":
		state.Suspended = false
		state.Repetitions = 0
		state.Lapses++
		state.IntervalDays = 1
		state.EaseFactor = clampEase(state.EaseFactor - 0.20)
	case "hard":
		state.Suspended = false
		state.Repetitions++
		state.IntervalDays = maxInt(1, int(math.Round(float64(maxInt(1, state.IntervalDays))*1.2)))
		state.EaseFactor = clampEase(state.EaseFactor - 0.15)
	case "good":
		state.Suspended = false
		state.Repetitions++
		switch state.Repetitions {
		case 1:
			state.IntervalDays = 1
		case 2:
			state.IntervalDays = 3
		default:
			state.IntervalDays = maxInt(1, int(math.Round(float64(maxInt(1, state.IntervalDays))*state.EaseFactor)))
		}
	case "easy":
		state.Suspended = false
		state.Repetitions++
		state.EaseFactor = clampEase(state.EaseFactor + 0.15)
		if state.Repetitions == 1 {
			state.IntervalDays = 4
		} else {
			state.IntervalDays = maxInt(1, int(math.Round(float64(maxInt(1, state.IntervalDays))*state.EaseFactor*1.3)))
		}
	case "archive":
		state.Suspended = true
		state.DueAt = nil
		return nil
	default:
		return errors.New("rating must be one of: again, hard, good, easy, archive")
	}

	due := now.AddDate(0, 0, state.IntervalDays)
	state.DueAt = &due
	return nil
}

func clampEase(v float64) float64 {
	if v < 1.3 {
		return 1.3
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
