package models

import (
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenDatabase opens a GORM database connection based on the DATABASE_URL.
// URLs starting with "postgres://" use PostgreSQL; everything else is treated as a SQLite file path.
func OpenDatabase(databaseURL string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
		dialector = postgres.Open(databaseURL)
	} else {
		// SQLite: add WAL mode and busy timeout for concurrent access
		dsn := databaseURL
		if !strings.Contains(dsn, "?") {
			dsn += "?"
		} else {
			dsn += "&"
		}
		dsn += "_journal_mode=WAL&_busy_timeout=5000"
		dialector = sqlite.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return db, nil
}

// AutoMigrate runs GORM auto-migration for all models.
// This is used for development and testing. Production uses goose migrations.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&User{},
		&APIToken{},
		&Source{},
		&Document{},
		&Highlight{},
		&ReviewState{},
		&Template{},
		&SyncLog{},
	); err != nil {
		return err
	}
	return backfill(db)
}

// backfill runs idempotent data migrations after the schema is in place.
func backfill(db *gorm.DB) error {
	// Favorite was historically tracked on ReviewState. It is now a first-class
	// field on Highlight. Copy any pre-existing favorites across. Safe to re-run:
	// it only flips highlights that are not already favorited.
	if db.Migrator().HasTable(&ReviewState{}) && db.Migrator().HasColumn(&ReviewState{}, "favorite") {
		if err := db.Exec(`
			UPDATE highlights
			SET favorite = ?
			WHERE favorite = ?
			  AND id IN (SELECT highlight_id FROM review_states WHERE favorite = ?)
		`, true, false, true).Error; err != nil {
			return fmt.Errorf("backfilling highlight favorites: %w", err)
		}
	}
	return nil
}
