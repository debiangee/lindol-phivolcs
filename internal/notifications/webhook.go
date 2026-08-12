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

// WebhookNotifier sends alerts to a generic webhook URL (POST JSON).
type WebhookNotifier struct {
	url    string
	client *http.Client
}

// NewWebhookNotifier creates a new generic webhook notifier.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled returns true if the webhook is configured.
func (w *WebhookNotifier) Enabled() bool {
	return w.url != ""
}

// WebhookPayload is the JSON payload sent to the webhook.
type WebhookPayload struct {
	Event      string            `json:"event"`
	Earthquake models.Earthquake `json:"earthquake"`
	Timestamp  string            `json:"timestamp"`
}

// SendInitialAlert sends the detection event to the webhook.
func (w *WebhookNotifier) SendInitialAlert(ctx context.Context, eq models.Earthquake) error {
	if !w.Enabled() {
		return nil
	}

	payload := WebhookPayload{
		Event:      "earthquake.detected",
		Earthquake: eq,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	return w.send(ctx, payload)
}

// SendEnrichmentUpdate sends the enrichment event to the webhook.
func (w *WebhookNotifier) SendEnrichmentUpdate(ctx context.Context, eq models.Earthquake) error {
	if !w.Enabled() {
		return nil
	}

	payload := WebhookPayload{
		Event:      "earthquake.enriched",
		Earthquake: eq,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	return w.send(ctx, payload)
}

func (w *WebhookNotifier) send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "lindol-api/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
