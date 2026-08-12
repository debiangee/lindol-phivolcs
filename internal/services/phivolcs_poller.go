package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"github.com/debiangee/lindol-api/internal/database"
	"github.com/debiangee/lindol-api/internal/models"
	"github.com/debiangee/lindol-api/internal/sources"
)

// PhivolcsPollerService polls PHIVOLCS as a primary source for earthquake detection.
type PhivolcsPollerService struct {
	db       *database.DB
	phivolcs *sources.PhivolcsClient
	notif    *NotificationService
	health   *HealthTracker
	logger   *slog.Logger
}

// NewPhivolcsPollerService creates a new PHIVOLCS poller service.
func NewPhivolcsPollerService(
	db *database.DB,
	phivolcs *sources.PhivolcsClient,
	notif *NotificationService,
	health *HealthTracker,
	logger *slog.Logger,
) *PhivolcsPollerService {
	return &PhivolcsPollerService{
		db:       db,
		phivolcs: phivolcs,
		notif:    notif,
		health:   health,
		logger:   logger,
	}
}

// PollAndDetect fetches from PHIVOLCS and returns newly detected earthquakes.
// Stops processing entries once it hits one that's already in the database (entries are ordered newest-first).
func (s *PhivolcsPollerService) PollAndDetect(ctx context.Context) ([]models.Earthquake, error) {
	result, err := s.phivolcs.FetchRecentEntries(ctx)
	if err != nil {
		s.health.PHIVOLCS.RecordError(err.Error())
		return nil, fmt.Errorf("fetch PHIVOLCS: %w", err)
	}

	s.health.PHIVOLCS.RecordSuccess()

	// Report transform errors
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

	// Process entries (newest first) and stop at the first known entry
	var newQuakes []models.Earthquake

	for _, entry := range result.Entries {
		id := generateID(entry)

		// Check if we already have this entry
		var exists int
		err := s.db.Conn.QueryRow("SELECT COUNT(*) FROM earthquakes WHERE id = ?", id).Scan(&exists)
		if err != nil {
			s.logger.Error("Failed to check earthquake", "id", id, "error", err)
			continue
		}
		if exists > 0 {
			// We've seen this one before — everything after is also old. Stop.
			s.logger.Debug("Hit known entry, stopping scrape", "id", id, "new_count", len(newQuakes))
			break
		}

		eq := models.Earthquake{
			ID:                  id,
			Magnitude:           entry.Magnitude,
			Latitude:            entry.Latitude,
			Longitude:           entry.Longitude,
			DepthKm:             entry.DepthKm,
			EventTime:           entry.DateTime.UTC(),
			LocationDescription: entry.Location,
			Enriched:            false,
			CreatedAt:           time.Now().UTC(),
			UpdatedAt:           time.Now().UTC(),
		}

		if entry.BulletinURL != "" {
			url := entry.BulletinURL
			eq.PhivolcsBulletinURL = &url
		}

		if err := s.insert(eq); err != nil {
			s.logger.Error("Failed to insert PHIVOLCS earthquake", "id", id, "error", err)
			continue
		}

		newQuakes = append(newQuakes, eq)
		s.logger.Info("🚨 New earthquake detected (PHIVOLCS)",
			"id", eq.ID,
			"magnitude", eq.Magnitude,
			"location", eq.LocationDescription,
			"depth_km", eq.DepthKm,
			"time", eq.EventTime.Format(time.RFC3339),
		)
	}

	return newQuakes, nil
}

// insert inserts an earthquake record.
func (s *PhivolcsPollerService) insert(eq models.Earthquake) error {
	bulletinURL := ""
	if eq.PhivolcsBulletinURL != nil {
		bulletinURL = *eq.PhivolcsBulletinURL
	}

	_, err := s.db.Conn.Exec(`
		INSERT INTO earthquakes (id, magnitude, latitude, longitude, depth_km, event_time, location_description, phivolcs_bulletin_url, enriched, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		eq.ID,
		eq.Magnitude,
		eq.Latitude,
		eq.Longitude,
		eq.DepthKm,
		eq.EventTime.Format(time.RFC3339),
		eq.LocationDescription,
		nullableString(bulletinURL),
		eq.CreatedAt.Format(time.RFC3339),
		eq.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	return nil
}

// generateID creates a deterministic unique ID for a PHIVOLCS entry.
func generateID(entry sources.PhivolcsEntry) string {
	data := fmt.Sprintf("%s|%.4f|%.4f|%.1f",
		entry.DateTime.UTC().Format(time.RFC3339),
		entry.Latitude,
		entry.Longitude,
		entry.Magnitude,
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("phivolcs_%x", hash[:8])
}
