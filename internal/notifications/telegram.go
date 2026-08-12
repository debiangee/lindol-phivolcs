// Package notifications handles alert dispatching to various channels.
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

// TelegramNotifier sends alerts via Telegram Bot API.
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramNotifier creates a new Telegram notifier.
func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled returns true if Telegram is configured.
func (t *TelegramNotifier) Enabled() bool {
	return t.botToken != "" && t.chatID != ""
}

// SendInitialAlert sends the first notification when a new earthquake is detected.
func (t *TelegramNotifier) SendInitialAlert(ctx context.Context, eq models.Earthquake) error {
	if !t.Enabled() {
		return nil
	}

	// Convert to PHT for display
	pht := time.FixedZone("PHT", 8*60*60)
	localTime := eq.EventTime.In(pht)

	message := fmt.Sprintf(
		"🚨 *Earthquake Detected*\n\n"+
			"*Magnitude:* %.1f\n"+
			"*Location:* %s\n"+
			"*Coordinates:* %.2f°N, %.2f°E\n"+
			"*Depth:* %.0f km\n"+
			"*Time:* %s\n"+
			"*Source:* USGS",
		eq.Magnitude,
		escapeMarkdown(eq.LocationDescription),
		eq.Latitude,
		eq.Longitude,
		eq.DepthKm,
		localTime.Format("02 Jan 2006 - 3:04 PM PHT"),
	)

	return t.sendMessage(ctx, message)
}

// SendEnrichmentUpdate sends a follow-up notification with PHIVOLCS data.
func (t *TelegramNotifier) SendEnrichmentUpdate(ctx context.Context, eq models.Earthquake) error {
	if !t.Enabled() {
		return nil
	}

	message := fmt.Sprintf("📋 *Update: M%.1f Earthquake*\n\n", eq.Magnitude)

	if eq.PhivolcsIntensity != nil && *eq.PhivolcsIntensity != "" {
		message += fmt.Sprintf("*Intensity:* %s\n", *eq.PhivolcsIntensity)
	}

	if len(eq.PhivolcsFeltAreas) > 0 {
		areas := ""
		for i, area := range eq.PhivolcsFeltAreas {
			if i > 0 {
				areas += ", "
			}
			areas += escapeMarkdown(area)
			if i >= 9 {
				areas += fmt.Sprintf(" (+%d more)", len(eq.PhivolcsFeltAreas)-10)
				break
			}
		}
		message += fmt.Sprintf("*Felt in:* %s\n", areas)
	}

	if eq.PhivolcsBulletinURL != nil && *eq.PhivolcsBulletinURL != "" {
		message += fmt.Sprintf("\n[View PHIVOLCS Bulletin](%s)", *eq.PhivolcsBulletinURL)
	}

	return t.sendMessage(ctx, message)
}

func (t *TelegramNotifier) sendMessage(ctx context.Context, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// escapeMarkdown escapes special characters for Telegram Markdown.
func escapeMarkdown(s string) string {
	replacer := []string{
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"`", "\\`",
	}
	r := s
	for i := 0; i < len(replacer); i += 2 {
		r = replaceAll(r, replacer[i], replacer[i+1])
	}
	return r
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if string(s[i]) == old {
			result += new
		} else {
			result += string(s[i])
		}
	}
	return result
}


// SendRawMessage sends a pre-formatted message (for dev alerts).
func (t *TelegramNotifier) SendRawMessage(ctx context.Context, message string) error {
	if !t.Enabled() {
		return nil
	}
	return t.sendMessage(ctx, message)
}
