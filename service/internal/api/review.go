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
	DueCount      int64 `json:"due_count"`
	NewCount      int64 `json:"new_count"`
	ReviewedToday int64 `json:"reviewed_today"`
}

type reviewCardData struct {
	Highlight  *models.Highlight
	State      *models.ReviewState
	Stats      reviewQueueStats
	LastAction string
	Error      string
}

type reviewResponse struct {
	Stats     reviewQueueStats    `json:"stats"`
	Highlight *models.Highlight   `json:"highlight,omitempty"`
	State     *models.ReviewState `json:"state,omitempty"`
}

type reviewActionRequest struct {
	Rating string `json:"rating"`
}

// --- Review API ---

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	highlight, state, err := s.nextDueReview(now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats, err := s.reviewStats(now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, reviewResponse{Stats: stats, Highlight: highlight, State: state})
}

func (s *Server) handleReviewAction(w http.ResponseWriter, r *http.Request) {
	highlightID := chi.URLParam(r, "id")
	var req reviewActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := s.applyReviewAction(highlightID, req.Rating, time.Now())
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, state)
}

// --- Review scheduling ---

func (s *Server) nextDueReview(now time.Time) (*models.Highlight, *models.ReviewState, error) {
	var highlight models.Highlight
	err := s.dueReviewQuery(now).
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

func (s *Server) reviewStats(now time.Time) (reviewQueueStats, error) {
	var stats reviewQueueStats
	if err := s.dueReviewQuery(now).Count(&stats.DueCount).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Highlight{}).
		Joins("LEFT JOIN review_states ON review_states.highlight_id = highlights.id").
		Where("review_states.highlight_id IS NULL").
		Count(&stats.NewCount).Error; err != nil {
		return stats, err
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if err := s.db.Model(&models.ReviewState{}).
		Where("last_reviewed_at >= ?", startOfDay).
		Count(&stats.ReviewedToday).Error; err != nil {
		return stats, err
	}

	return stats, nil
}

func (s *Server) dueReviewQuery(now time.Time) *gorm.DB {
	return s.db.Model(&models.Highlight{}).
		Joins("LEFT JOIN review_states ON review_states.highlight_id = highlights.id").
		Where("review_states.highlight_id IS NULL OR (review_states.suspended = ? AND review_states.due_at <= ?)", false, now)
}

func (s *Server) applyReviewAction(highlightID, rating string, now time.Time) (*models.ReviewState, error) {
	if rating == "" {
		return nil, errors.New("rating is required")
	}

	var highlight models.Highlight
	if err := s.db.First(&highlight, "id = ?", highlightID).Error; err != nil {
		return nil, err
	}

	var state models.ReviewState
	err := s.db.First(&state, "highlight_id = ?", highlightID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state = models.ReviewState{HighlightID: highlightID, EaseFactor: defaultEaseFactor}
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
