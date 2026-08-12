// Package sources handles data fetching from external earthquake sources.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/debiangee/lindol-api/internal/config"
	"github.com/debiangee/lindol-api/internal/models"
)

const usgsBaseURL = "https://earthquake.usgs.gov/fdsnws/event/1/query"

// USGSClient fetches earthquake data from the USGS API.
type USGSClient struct {
	cfg    *config.Config
	client *http.Client
	logger *slog.Logger
}

// NewUSGSClient creates a new USGS API client.
func NewUSGSClient(cfg *config.Config, logger *slog.Logger) *USGSClient {
	return &USGSClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// FetchRecent fetches earthquakes from the last N minutes within the PH bounding box.
func (u *USGSClient) FetchRecent(ctx context.Context) ([]models.Earthquake, error) {
	// Look back a window slightly larger than poll interval to avoid missing events
	startTime := time.Now().UTC().Add(-10 * time.Minute)

	url := fmt.Sprintf(
		"%s?format=geojson&starttime=%s&minlatitude=%.1f&maxlatitude=%.1f&minlongitude=%.1f&maxlongitude=%.1f&minmagnitude=%.1f&orderby=time",
		usgsBaseURL,
		startTime.Format("2006-01-02T15:04:05"),
		u.cfg.RegionMinLat,
		u.cfg.RegionMaxLat,
		u.cfg.RegionMinLon,
		u.cfg.RegionMaxLon,
		u.cfg.MinMagnitude,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "lindol-api/1.0 (earthquake monitoring)")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch USGS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("USGS returned status %d", resp.StatusCode)
	}

	var collection models.USGSFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("decode USGS response: %w", err)
	}

	earthquakes := make([]models.Earthquake, 0, len(collection.Features))
	for i := range collection.Features {
		eq := collection.Features[i].ToEarthquake()
		earthquakes = append(earthquakes, eq)
	}

	u.logger.Debug("USGS fetch complete",
		"count", len(earthquakes),
		"startTime", startTime.Format(time.RFC3339),
	)

	return earthquakes, nil
}
