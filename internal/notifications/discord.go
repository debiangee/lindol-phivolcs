package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/debiangee/lindol-api/internal/models"
)

// DiscordNotifier sends alerts via Discord webhooks.
type DiscordNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewDiscordNotifier creates a new Discord notifier.
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled returns true if Discord is configured.
func (d *DiscordNotifier) Enabled() bool {
	return d.webhookURL != ""
}

// SendInitialAlert sends the first notification when a new earthquake is detected.
func (d *DiscordNotifier) SendInitialAlert(ctx context.Context, eq models.Earthquake) error {
	if !d.Enabled() {
		return nil
	}

	pht := time.FixedZone("PHT", 8*60*60)
	localTime := eq.EventTime.In(pht)

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("🚨 M%.1f Earthquake Detected", eq.Magnitude),
		"color":       getColorForMagnitude(eq.Magnitude),
		"description": eq.LocationDescription,
		"fields": []map[string]interface{}{
			{"name": "Magnitude", "value": fmt.Sprintf("%.1f", eq.Magnitude), "inline": true},
			{"name": "Depth", "value": fmt.Sprintf("%.0f km", eq.DepthKm), "inline": true},
			{"name": "Coordinates", "value": fmt.Sprintf("%.2f°N, %.2f°E", eq.Latitude, eq.Longitude), "inline": true},
			{"name": "Time (PHT)", "value": localTime.Format("02 Jan 2006 - 3:04 PM"), "inline": false},
		},
		"footer": map[string]string{"text": "Source: USGS • lindol-api"},
		"timestamp": eq.EventTime.Format(time.RFC3339),
	}

	payload := map[string]interface{}{
		"embeds": []interface{}{embed},
	}

	return d.send(ctx, payload)
}

// SendEnrichmentUpdate sends a follow-up with PHIVOLCS data.
func (d *DiscordNotifier) SendEnrichmentUpdate(ctx context.Context, eq models.Earthquake) error {
	if !d.Enabled() {
		return nil
	}

	fields := []map[string]interface{}{}

	if eq.PhivolcsIntensity != nil && *eq.PhivolcsIntensity != "" {
		fields = append(fields, map[string]interface{}{
			"name": "Intensity", "value": *eq.PhivolcsIntensity, "inline": true,
		})
	}

	if len(eq.PhivolcsFeltAreas) > 0 {
		areas := ""
		for i, area := range eq.PhivolcsFeltAreas {
			if i > 0 {
				areas += ", "
			}
			areas += area
			if i >= 9 {
				areas += fmt.Sprintf(" (+%d more)", len(eq.PhivolcsFeltAreas)-10)
				break
			}
		}
		fields = append(fields, map[string]interface{}{
			"name": "Felt Areas", "value": areas, "inline": false,
		})
	}

	description := ""
	if eq.PhivolcsBulletinURL != nil && *eq.PhivolcsBulletinURL != "" {
		description = fmt.Sprintf("[View PHIVOLCS Bulletin](%s)", *eq.PhivolcsBulletinURL)
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("📋 Update: M%.1f Earthquake", eq.Magnitude),
		"color":       0x3498DB,
		"description": description,
		"fields":      fields,
		"footer":      map[string]string{"text": "Enriched via PHIVOLCS • lindol-api"},
	}

	payload := map[string]interface{}{
		"embeds": []interface{}{embed},
	}

	return d.send(ctx, payload)
}

func (d *DiscordNotifier) send(ctx context.Context, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("send discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// getColorForMagnitude returns a Discord embed color based on earthquake severity.
func getColorForMagnitude(mag float64) int {
	switch {
	case mag >= 7.0:
		return 0x8B0000 // Dark red
	case mag >= 6.0:
		return 0xFF0000 // Red
	case mag >= 5.0:
		return 0xFF6600 // Orange
	case mag >= 4.0:
		return 0xFFCC00 // Yellow
	default:
		return 0x00CC00 // Green
	}
}


// SendDevAlert sends a parser/developer alert to Discord.
func (d *DiscordNotifier) SendDevAlert(ctx context.Context, field, rawValue, errMsg string) error {
	if !d.Enabled() {
		return nil
	}

	embed := map[string]interface{}{
		"title": "⚠️ Parser Alert",
		"color": 0xFF9900,
		"fields": []map[string]interface{}{
			{"name": "Field", "value": field, "inline": true},
			{"name": "Raw Value", "value": "`" + rawValue + "`", "inline": true},
			{"name": "Error", "value": errMsg, "inline": false},
		},
		"footer": map[string]string{"text": "Check if PHIVOLCS changed their HTML format"},
	}

	payload := map[string]interface{}{
		"embeds": []interface{}{embed},
	}

	return d.send(ctx, payload)
}
