// Package services contains core business logic.
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/debiangee/lindol-api/internal/database"
	"github.com/debiangee/lindol-api/internal/models"
	"github.com/debiangee/lindol-api/internal/sources"
)

// EarthquakeService handles earthquake detection, storage, and retrieval.
type EarthquakeService struct {
	db     *database.DB
	usgs   *sources.USGSClient
	logger *slog.Logger
}

// NewEarthquakeService creates a new EarthquakeService.
func NewEarthquakeService(db *database.DB, usgs *sources.USGSClient, logger *slog.Logger) *EarthquakeService {
	return &EarthquakeService{
		db:     db,
		usgs:   usgs,
		logger: logger,
	}
}

// DB returns the database reference (for status queries).
func (s *EarthquakeService) DB() *database.DB {
	return s.db
}

// PollAndDetect fetches from USGS and returns newly detected earthquakes.
func (s *EarthquakeService) PollAndDetect(ctx context.Context) ([]models.Earthquake, error) {
	earthquakes, err := s.usgs.FetchRecent(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch from USGS: %w", err)
	}

	var newQuakes []models.Earthquake

	for _, eq := range earthquakes {
		isNew, err := s.insertIfNew(eq)
		if err != nil {
			s.logger.Error("Failed to insert earthquake", "id", eq.ID, "error", err)
			continue
		}
		if isNew {
			newQuakes = append(newQuakes, eq)
			s.logger.Info("🚨 New earthquake detected",
				"id", eq.ID,
				"magnitude", eq.Magnitude,
				"location", eq.LocationDescription,
				"depth_km", eq.DepthKm,
				"time", eq.EventTime.Format(time.RFC3339),
			)
		}
	}

	return newQuakes, nil
}

// insertIfNew inserts an earthquake if it doesn't already exist. Returns true if inserted.
func (s *EarthquakeService) insertIfNew(eq models.Earthquake) (bool, error) {
	// Check if already exists
	var exists int
	err := s.db.Conn.QueryRow("SELECT COUNT(*) FROM earthquakes WHERE id = ?", eq.ID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check exists: %w", err)
	}
	if exists > 0 {
		return false, nil
	}

	// Insert new record
	_, err = s.db.Conn.Exec(`
		INSERT INTO earthquakes (id, magnitude, latitude, longitude, depth_km, event_time, location_description, enriched, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		eq.ID,
		eq.Magnitude,
		eq.Latitude,
		eq.Longitude,
		eq.DepthKm,
		eq.EventTime.Format(time.RFC3339),
		eq.LocationDescription,
		eq.CreatedAt.Format(time.RFC3339),
		eq.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return false, fmt.Errorf("insert: %w", err)
	}

	return true, nil
}

// GetByID retrieves a single earthquake by its ID.
func (s *EarthquakeService) GetByID(id string) (*models.Earthquake, error) {
	row := s.db.Conn.QueryRow(`
		SELECT id, magnitude, latitude, longitude, depth_km, event_time, location_description,
		       phivolcs_intensity, phivolcs_felt_areas, phivolcs_bulletin_url, enriched, created_at, updated_at
		FROM earthquakes WHERE id = ?`, id)

	return scanEarthquake(row)
}

// GetLatest retrieves the most recent earthquake.
func (s *EarthquakeService) GetLatest() (*models.Earthquake, error) {
	row := s.db.Conn.QueryRow(`
		SELECT id, magnitude, latitude, longitude, depth_km, event_time, location_description,
		       phivolcs_intensity, phivolcs_felt_areas, phivolcs_bulletin_url, enriched, created_at, updated_at
		FROM earthquakes ORDER BY event_time DESC LIMIT 1`)

	return scanEarthquake(row)
}

// ListFilters holds query parameters for listing earthquakes.
type ListFilters struct {
	MinMagnitude *float64
	MaxMagnitude *float64
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}

// List retrieves earthquakes with optional filters.
func (s *EarthquakeService) List(filters ListFilters) ([]models.Earthquake, int, error) {
	query := "SELECT id, magnitude, latitude, longitude, depth_km, event_time, location_description, phivolcs_intensity, phivolcs_felt_areas, phivolcs_bulletin_url, enriched, created_at, updated_at FROM earthquakes WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM earthquakes WHERE 1=1"
	var args []interface{}

	if filters.MinMagnitude != nil {
		query += " AND magnitude >= ?"
		countQuery += " AND magnitude >= ?"
		args = append(args, *filters.MinMagnitude)
	}
	if filters.MaxMagnitude != nil {
		query += " AND magnitude <= ?"
		countQuery += " AND magnitude <= ?"
		args = append(args, *filters.MaxMagnitude)
	}
	if filters.StartDate != nil {
		query += " AND event_time >= ?"
		countQuery += " AND event_time >= ?"
		args = append(args, filters.StartDate.Format(time.RFC3339))
	}
	if filters.EndDate != nil {
		query += " AND event_time <= ?"
		countQuery += " AND event_time <= ?"
		args = append(args, filters.EndDate.Format(time.RFC3339))
	}

	// Get total count
	var total int
	if err := s.db.Conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY event_time DESC LIMIT ? OFFSET ?"
	args = append(args, filters.Limit, filters.Offset)

	rows, err := s.db.Conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var earthquakes []models.Earthquake
	for rows.Next() {
		eq, err := scanEarthquakeRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		earthquakes = append(earthquakes, *eq)
	}

	return earthquakes, total, nil
}

func scanEarthquake(row *sql.Row) (*models.Earthquake, error) {
	var eq models.Earthquake
	var eventTime, createdAt, updatedAt string
	var intensity, feltAreas, bulletinURL sql.NullString
	var enriched int

	err := row.Scan(
		&eq.ID, &eq.Magnitude, &eq.Latitude, &eq.Longitude, &eq.DepthKm,
		&eventTime, &eq.LocationDescription,
		&intensity, &feltAreas, &bulletinURL, &enriched,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	eq.EventTime, _ = time.Parse(time.RFC3339, eventTime)
	eq.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	eq.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	eq.Enriched = enriched == 1

	if intensity.Valid {
		eq.PhivolcsIntensity = &intensity.String
	}
	if bulletinURL.Valid {
		eq.PhivolcsBulletinURL = &bulletinURL.String
	}
	if feltAreas.Valid && feltAreas.String != "" {
		// Parse JSON array
		eq.PhivolcsFeltAreas = parseJSONArray(feltAreas.String)
	}

	return &eq, nil
}

func scanEarthquakeRow(rows *sql.Rows) (*models.Earthquake, error) {
	var eq models.Earthquake
	var eventTime, createdAt, updatedAt string
	var intensity, feltAreas, bulletinURL sql.NullString
	var enriched int

	err := rows.Scan(
		&eq.ID, &eq.Magnitude, &eq.Latitude, &eq.Longitude, &eq.DepthKm,
		&eventTime, &eq.LocationDescription,
		&intensity, &feltAreas, &bulletinURL, &enriched,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	eq.EventTime, _ = time.Parse(time.RFC3339, eventTime)
	eq.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	eq.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	eq.Enriched = enriched == 1

	if intensity.Valid {
		eq.PhivolcsIntensity = &intensity.String
	}
	if bulletinURL.Valid {
		eq.PhivolcsBulletinURL = &bulletinURL.String
	}
	if feltAreas.Valid && feltAreas.String != "" {
		eq.PhivolcsFeltAreas = parseJSONArray(feltAreas.String)
	}

	return &eq, nil
}

func parseJSONArray(s string) []string {
	var result []string
	if len(s) > 2 && s[0] == '[' {
		data := []byte(s)
		_ = json.Unmarshal(data, &result)
	}
	return result
}
