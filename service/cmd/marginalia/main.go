package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/adampetrovic/marginalia/service/internal/api"
	"github.com/adampetrovic/marginalia/service/internal/config"
	"github.com/adampetrovic/marginalia/service/internal/models"
	"github.com/adampetrovic/marginalia/service/internal/sync/readeck"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := models.OpenDatabase(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	// Auto-migrate for development; production should use goose
	if err := models.AutoMigrate(db); err != nil {
		logger.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	// Ensure sources exist for configured integrations
	if cfg.IsReadeckConfigured() {
		if _, err := readeck.EnsureSource(db); err != nil {
			logger.Error("failed to ensure readeck source", "error", err)
			os.Exit(1)
		}
		logger.Info("readeck source configured", "url", cfg.ReadeckURL)
	}

	srv := api.NewServer(cfg, db, logger)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("starting marginalia", "addr", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
