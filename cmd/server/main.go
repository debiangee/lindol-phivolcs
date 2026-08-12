package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/debiangee/lindol-api/internal/config"
	"github.com/debiangee/lindol-api/internal/database"
	"github.com/debiangee/lindol-api/internal/notifications"
	"github.com/debiangee/lindol-api/internal/server"
	"github.com/debiangee/lindol-api/internal/services"
	"github.com/debiangee/lindol-api/internal/sources"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup structured logger
	var handler slog.Handler
	if cfg.Env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Initialize database
	dbPath := filepath.Join("data", "lindol.db")
	db, err := database.New(dbPath, logger)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize sources
	usgsClient := sources.NewUSGSClient(cfg, logger)
	phivolcsClient := sources.NewPhivolcsClient(logger)

	// Initialize notifications
	telegramNotifier := notifications.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID)
	discordNotifier := notifications.NewDiscordNotifier(cfg.DiscordWebhookURL)
	webhookNotifier := notifications.NewWebhookNotifier(cfg.WebhookURL)

	// Initialize services
	healthTracker := services.NewHealthTracker()
	notifService := services.NewNotificationService(telegramNotifier, discordNotifier, webhookNotifier, logger)
	eqService := services.NewEarthquakeService(db, usgsClient, logger)
	enrichService := services.NewEnrichmentService(db, phivolcsClient, cfg, logger, notifService)
	phivolcsPoller := services.NewPhivolcsPollerService(db, phivolcsClient, notifService, healthTracker, logger)

	// Log notification status
	if telegramNotifier.Enabled() {
		logger.Info("Telegram notifications enabled")
	}
	if discordNotifier.Enabled() {
		logger.Info("Discord notifications enabled")
	}
	if webhookNotifier.Enabled() {
		logger.Info("Webhook notifications enabled")
	}

	// Create and start server
	srv := server.New(cfg, logger, eqService, healthTracker)
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Context for background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start pollers
	go startUSGSPoller(ctx, cfg, eqService, enrichService, notifService, healthTracker, logger)

	if cfg.PhivolcsPrimary {
		go startPhivolcsPoller(ctx, cfg, phivolcsPoller, notifService, logger)
		logger.Info("PHIVOLCS primary poller enabled", "interval_sec", cfg.PhivolcsPollIntervalSec)
	}

	// Start HTTP server
	go func() {
		logger.Info("🌋 Lindol API running", "port", cfg.Port, "env", cfg.Env)
		logger.Info("Health check", "url", "http://localhost:"+cfg.Port+"/api/health")
		logger.Info("USGS poller started", "interval_sec", cfg.USGSPollIntervalSec)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped")
}

// startUSGSPoller runs the USGS polling loop.
func startUSGSPoller(
	ctx context.Context,
	cfg *config.Config,
	eqService *services.EarthquakeService,
	enrichService *services.EnrichmentService,
	notifService *services.NotificationService,
	health *services.HealthTracker,
	logger *slog.Logger,
) {
	ticker := time.NewTicker(time.Duration(cfg.USGSPollIntervalSec) * time.Second)
	defer ticker.Stop()

	pollUSGS(ctx, eqService, enrichService, notifService, health, logger)

	for {
		select {
		case <-ctx.Done():
			logger.Info("USGS poller stopped")
			return
		case <-ticker.C:
			pollUSGS(ctx, eqService, enrichService, notifService, health, logger)
		}
	}
}

func pollUSGS(
	ctx context.Context,
	eqService *services.EarthquakeService,
	enrichService *services.EnrichmentService,
	notifService *services.NotificationService,
	health *services.HealthTracker,
	logger *slog.Logger,
) {
	newQuakes, err := eqService.PollAndDetect(ctx)
	if err != nil {
		logger.Error("USGS poll failed", "error", err)
		health.USGS.RecordError(err.Error())
		return
	}

	health.USGS.RecordSuccess()

	for _, eq := range newQuakes {
		notifService.NotifyNewEarthquake(ctx, eq)
		go enrichService.ScheduleEnrichment(ctx, eq)
	}
}

// startPhivolcsPoller runs the PHIVOLCS primary polling loop.
func startPhivolcsPoller(
	ctx context.Context,
	cfg *config.Config,
	poller *services.PhivolcsPollerService,
	notifService *services.NotificationService,
	logger *slog.Logger,
) {
	ticker := time.NewTicker(time.Duration(cfg.PhivolcsPollIntervalSec) * time.Second)
	defer ticker.Stop()

	// Run immediately
	pollPhivolcs(ctx, poller, notifService, logger)

	for {
		select {
		case <-ctx.Done():
			logger.Info("PHIVOLCS poller stopped")
			return
		case <-ticker.C:
			pollPhivolcs(ctx, poller, notifService, logger)
		}
	}
}

func pollPhivolcs(
	ctx context.Context,
	poller *services.PhivolcsPollerService,
	notifService *services.NotificationService,
	logger *slog.Logger,
) {
	newQuakes, err := poller.PollAndDetect(ctx)
	if err != nil {
		logger.Error("PHIVOLCS poll failed", "error", err)
		return
	}

	for _, eq := range newQuakes {
		notifService.NotifyNewEarthquake(ctx, eq)
	}
}
