package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/debiangee/lindol-api/internal/config"
	"github.com/debiangee/lindol-api/internal/database"
	"github.com/debiangee/lindol-api/internal/models"
	"github.com/debiangee/lindol-api/internal/sources"
)

// EnrichmentService handles PHIVOLCS enrichment of earthquake records.
type EnrichmentService struct {
	db       *database.DB
	phivolcs *sources.PhivolcsClient
	cfg      *config.Config
	logger   *slog.Logger
	notif    *NotificationService
}

// NewEnrichmentService creates a new EnrichmentService.
func NewEnrichmentService(db *database.DB, phivolcs *sources.PhivolcsClient, cfg *config.Config, logger *slog.Logger, notif *NotificationService) *EnrichmentService {
	return &EnrichmentService{
		db:       db,
		phivolcs: phivolcs,
		cfg:      cfg,
		logger:   logger,
		notif:    notif,
	}
}

// ScheduleEnrichment waits for the configured delay then enriches the given earthquake.
func (s *EnrichmentService) ScheduleEnrichment(ctx context.Context, eq models.Earthquake) {
	delay := time.Duration(s.cfg.PhivolcsDelaySec) * time.Second
	s.logger.Info("Enrichment scheduled",
		"id", eq.ID,
		"delay_sec", s.cfg.PhivolcsDelaySec,
		"magnitude", eq.Magnitude,
	)

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		s.enrich(ctx, eq)
	}
}

// enrich attempts to find and apply PHIVOLCS data to an earthquake record.
func (s *EnrichmentService) enrich(ctx context.Context, eq models.Earthquake) {
	s.logger.Info("Starting PHIVOLCS enrichment", "id", eq.ID)

	// Fetch recent entries from PHIVOLCS
	result, err := s.phivolcs.FetchRecentEntries(ctx)
	if err != nil {
		s.logger.Error("PHIVOLCS fetch failed", "id", eq.ID, "error", err)
		return
	}

	// Report any transform errors
	for _, tErr := range result.TransformErrors {
		s.logger.Warn("PHIVOLCS transform error",
			"field", tErr.Field,
			"raw_value", tErr.RawValue,
			"error", tErr.Err,
		)
		if s.notif != nil {
			s.notif.NotifyTransformError(ctx, tErr.Field, tErr.RawValue, tErr.Err.Error())
		}
	}

	if len(result.Entries) == 0 {
		s.logger.Warn("No PHIVOLCS entries found", "id", eq.ID)
		return
	}

	// Match by time and location proximity
	match := sources.MatchEntry(
		result.Entries,
		eq.EventTime,
		eq.Latitude,
		eq.Longitude,
		s.cfg.PhivolcsMatchTimeMin,
		s.cfg.PhivolcsMatchDistDeg,
	)

	if match == nil {
		s.logger.Info("No PHIVOLCS match found", "id", eq.ID)
		return
	}

	s.logger.Info("PHIVOLCS match found",
		"id", eq.ID,
		"bulletin", match.BulletinURL,
		"phivolcs_mag", match.Magnitude,
	)

	// Fetch bulletin for intensity details
	var intensity string
	var feltAreas []string

	if match.BulletinURL != "" {
		bulletin, err := s.phivolcs.FetchBulletin(ctx, match.BulletinURL)
		if err != nil {
			s.logger.Warn("Failed to fetch bulletin", "id", eq.ID, "url", match.BulletinURL, "error", err)
		} else {
			intensity = bulletin.Intensity
			feltAreas = bulletin.FeltAreas
		}
	}

	// Update database record
	if err := s.updateRecord(eq.ID, intensity, feltAreas, match.BulletinURL); err != nil {
		s.logger.Error("Failed to update enrichment", "id", eq.ID, "error", err)
		return
	}

	s.logger.Info("Earthquake enriched",
		"id", eq.ID,
		"intensity", intensity,
		"felt_areas_count", len(feltAreas),
	)
}

// updateRecord updates the earthquake record with PHIVOLCS data.
func (s *EnrichmentService) updateRecord(id string, intensity string, feltAreas []string, bulletinURL string) error {
	var feltAreasJSON *string
	if len(feltAreas) > 0 {
		data, err := json.Marshal(feltAreas)
		if err == nil {
			str := string(data)
			feltAreasJSON = &str
		}
	}

	fullURL := bulletinURL
	if fullURL != "" && !isAbsoluteURL(fullURL) {
		fullURL = "https://earthquake.phivolcs.dost.gov.ph/" + fullURL
	}
	fullURL = strings.ReplaceAll(fullURL, "\\", "/")

	_, err := s.db.Conn.Exec(`
		UPDATE earthquakes
		SET phivolcs_intensity = ?,
		    phivolcs_felt_areas = ?,
		    phivolcs_bulletin_url = ?,
		    enriched = 1,
		    updated_at = ?
		WHERE id = ?`,
		nullableString(intensity),
		feltAreasJSON,
		nullableString(fullURL),
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("update record: %w", err)
	}

	return nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isAbsoluteURL(s string) bool {
	return len(s) > 4 && s[:4] == "http"
}
