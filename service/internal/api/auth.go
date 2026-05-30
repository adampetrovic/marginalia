package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/adampetrovic/marginalia/service/internal/auth"
	"github.com/adampetrovic/marginalia/service/internal/models"
)

const sessionCookie = "marginalia_session"

type ctxKey int

const userCtxKey ctxKey = iota

// userFromCtx returns the authenticated user attached to the request, if any.
func userFromCtx(r *http.Request) (*models.User, bool) {
	u, ok := r.Context().Value(userCtxKey).(*models.User)
	return u, ok
}

// currentUser returns the authenticated user; handlers behind authMiddleware can
// rely on it being non-nil.
func (s *Server) currentUser(r *http.Request) *models.User {
	u, _ := userFromCtx(r)
	return u
}

// scoped returns a query scoped to the authenticated user via user_id. A fresh
// statement is started on each call so it is safe to call multiple times per
// request.
func (s *Server) scoped(r *http.Request) *gorm.DB {
	return s.db.Where("user_id = ?", s.currentUser(r).ID)
}

// authMiddleware resolves the request's user from a session cookie or a bearer
// API token and stores it in the request context. Requests without a valid
// credential get 401.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.resolveUser(w, r)
		if user == nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveUser tries the bearer API token first (machine clients), then the
// session cookie (web UI). Returns nil if neither yields a valid user.
func (s *Server) resolveUser(w http.ResponseWriter, r *http.Request) *models.User {
	// 1. Bearer / Token Authorization header → API token.
	if authz := r.Header.Get("Authorization"); authz != "" {
		raw := authz
		for _, prefix := range []string{"Bearer ", "Token "} {
			if strings.HasPrefix(raw, prefix) {
				raw = raw[len(prefix):]
				break
			}
		}
		if u := s.userForAPIToken(raw); u != nil {
			return u
		}
	}

	// 2. Session cookie (web UI).
	if c, err := r.Cookie(sessionCookie); err == nil {
		if uid, err := auth.ParseSession(s.sessionSecret, c.Value, time.Now()); err == nil {
			var u models.User
			if err := s.db.First(&u, "id = ?", uid).Error; err == nil {
				return &u
			}
		}
	}

	return nil
}

// userForAPIToken looks up the user owning the given plaintext API token and
// records its last-used time. Returns nil if the token is unknown.
func (s *Server) userForAPIToken(plaintext string) *models.User {
	if plaintext == "" {
		return nil
	}
	hash := auth.HashAPIToken(plaintext)
	var tok models.APIToken
	if err := s.db.First(&tok, "token_hash = ?", hash).Error; err != nil {
		return nil
	}
	now := time.Now()
	s.db.Model(&tok).Update("last_used_at", &now)

	var u models.User
	if err := s.db.First(&u, "id = ?", tok.UserID).Error; err != nil {
		return nil
	}
	return &u
}

func (s *Server) setSessionCookie(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    auth.SignSession(s.sessionSecret, userID, time.Now()),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionTTL.Seconds()),
	})
}

// --- Auth handlers ---

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DisableRegistration {
		writeError(w, http.StatusForbidden, "registration is disabled")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	var count int64
	s.db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		writeError(w, http.StatusConflict, "an account with that email already exists")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	// The very first account becomes the admin.
	var total int64
	s.db.Model(&models.User{}).Count(&total)

	user := models.User{
		ID:           models.NewID("user"),
		Email:        req.Email,
		Name:         strings.TrimSpace(req.Name),
		PasswordHash: hash,
		IsAdmin:      total == 0,
	}
	if err := s.db.Create(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.setSessionCookie(w, user.ID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"user": user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	if err := s.db.First(&user, "email = ?", req.Email).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	s.setSessionCookie(w, user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": s.currentUser(r)})
}

// --- API token handlers ---

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	var tokens []models.APIToken
	s.scoped(r).Order("created_at DESC").Find(&tokens)
	if tokens == nil {
		tokens = []models.APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "API token"
	}

	plaintext, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	tok := models.APIToken{
		ID:        models.NewID("tok"),
		UserID:    s.currentUser(r).ID,
		Name:      req.Name,
		TokenHash: hash,
		Prefix:    prefix,
	}
	if err := s.db.Create(&tok).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// The plaintext token is returned exactly once.
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         tok.ID,
		"name":       tok.Name,
		"prefix":     tok.Prefix,
		"created_at": tok.CreatedAt,
		"token":      plaintext,
	})
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res := s.scoped(r).Where("id = ?", id).Delete(&models.APIToken{})
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
