package api

import (
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"github.com/adampetrovic/marginalia/service/internal/auth"
	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/sync/readeck"
)

// Bootstrap initialises the first admin account on a fresh database and adopts
// any data left over from the pre-multi-user (single shared token) era. It is a
// no-op once at least one user exists.
func Bootstrap(db *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount > 0 {
		return nil
	}

	hasAdminCreds := cfg.AdminEmail != "" && cfg.AdminPassword != ""
	if !hasAdminCreds && cfg.APIToken == "" {
		logger.Info("no accounts yet — create the first one via the web UI (it becomes admin)")
		return nil
	}

	// Determine the admin credentials. When only a legacy API token is provided
	// (e.g. an existing KOReader-only deployment), create an admin with a random
	// password so the token can be migrated; the operator can set
	// MARGINALIA_ADMIN_PASSWORD later to log into the web UI.
	email := strings.ToLower(strings.TrimSpace(cfg.AdminEmail))
	if email == "" {
		email = "admin@marginalia.local"
	}
	password := cfg.AdminPassword
	if password == "" {
		generated, err := auth.RandomSecret()
		if err != nil {
			return err
		}
		password = generated
		logger.Warn("created admin with a random password; set MARGINALIA_ADMIN_PASSWORD to log into the web UI", "email", email)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin := models.User{
		ID:           models.NewID("user"),
		Email:        email,
		Name:         "Admin",
		PasswordHash: hash,
		IsAdmin:      true,
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	logger.Info("created bootstrap admin account", "email", admin.Email)

	// Adopt any pre-existing single-user data so it isn't orphaned.
	for _, m := range []interface{}{
		&models.Source{}, &models.Document{}, &models.Highlight{},
		&models.ReviewState{}, &models.Template{}, &models.SyncLog{},
	} {
		db.Model(m).Where("user_id IS NULL OR user_id = ?", "").Update("user_id", admin.ID)
	}

	// Migrate the legacy shared API token so already-configured KOReader/Readest
	// devices keep authenticating.
	if cfg.APIToken != "" {
		prefix := cfg.APIToken
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		db.Create(&models.APIToken{
			ID:        models.NewID("tok"),
			UserID:    admin.ID,
			Name:      "Legacy token",
			TokenHash: auth.HashAPIToken(cfg.APIToken),
			Prefix:    prefix,
		})
		logger.Info("migrated legacy MARGINALIA_API_TOKEN to the admin account")
	}

	// Seed the admin's Readeck integration from environment configuration.
	if cfg.ReadeckURL != "" && cfg.ReadeckToken != "" {
		if src, err := readeck.EnsureSource(db, admin.ID); err == nil {
			src.Config = models.JSONMap{"url": cfg.ReadeckURL, "token": cfg.ReadeckToken}
			db.Model(src).Update("config", src.Config)
		}
	}

	return nil
}
