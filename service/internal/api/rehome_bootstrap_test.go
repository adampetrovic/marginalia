package api

import (
	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
	"os"
	"testing"
)

func TestBootstrapRehomesLegacyData(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	models.AutoMigrate(db)
	// Pre-multi-user data: legacy singleton source + doc, no user_id.
	db.Create(&models.Source{ID: "readeck", Type: "readeck", Name: "Readeck"})
	db.Create(&models.Document{ID: "readeck-bm1", SourceID: "readeck", SourceDocumentID: "bm1", Type: "article", Title: "Old", Tags: models.JSONStringArray{}, Metadata: models.JSONMap{}})
	db.Create(&models.SyncLog{SourceID: "readeck", Status: "completed"})
	cfg := &config.Config{APIToken: "legacy-tok", ReadeckURL: "http://r", ReadeckToken: "rt"}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := Bootstrap(db, cfg, log); err != nil {
		t.Fatal(err)
	}
	var admin models.User
	db.First(&admin)
	// Legacy source should be renamed; doc + synclog repointed; no duplicate source.
	var srcCount int64
	db.Model(&models.Source{}).Count(&srcCount)
	if srcCount != 1 {
		t.Fatalf("expected 1 source after re-home, got %d", srcCount)
	}
	var doc models.Document
	db.First(&doc, "id = ?", "readeck-bm1")
	if doc.SourceID != "readeck-"+admin.ID {
		t.Errorf("doc not repointed: %s", doc.SourceID)
	}
	if doc.UserID != admin.ID {
		t.Errorf("doc not adopted: %s", doc.UserID)
	}
	var src models.Source
	if err := db.First(&src, "id = ?", "readeck-"+admin.ID).Error; err != nil {
		t.Fatalf("renamed source missing: %v", err)
	}
	if url, _ := src.Config["url"].(string); url != "http://r" {
		t.Errorf("readeck config not seeded: %v", src.Config)
	}
}
