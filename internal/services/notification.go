package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/debiangee/lindol-api/internal/models"
	"github.com/debiangee/lindol-api/internal/notifications"
)

// NotificationService dispatches alerts to all configured channels.
type NotificationService struct {
	telegram *notifications.TelegramNotifier
	discord  *notifications.DiscordNotifier
	webhook  *notifications.WebhookNotifier
	logger   *slog.Logger
}

// NewNotificationService creates a new NotificationService with all configured notifiers.
func NewNotificationService(
	telegram *notifications.TelegramNotifier,
	discord *notifications.DiscordNotifier,
	webhook *notifications.WebhookNotifier,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		telegram: telegram,
		discord:  discord,
		webhook:  webhook,
		logger:   logger,
	}
}

// NotifyNewEarthquake sends initial alerts to all configured channels.
func (n *NotificationService) NotifyNewEarthquake(ctx context.Context, eq models.Earthquake) {
	if n.telegram.Enabled() {
		if err := n.telegram.SendInitialAlert(ctx, eq); err != nil {
			n.logger.Error("Telegram notification failed", "id", eq.ID, "error", err)
		} else {
			n.logger.Info("Telegram alert sent", "id", eq.ID)
		}
	}

	if n.discord.Enabled() {
		if err := n.discord.SendInitialAlert(ctx, eq); err != nil {
			n.logger.Error("Discord notification failed", "id", eq.ID, "error", err)
		} else {
			n.logger.Info("Discord alert sent", "id", eq.ID)
		}
	}

	if n.webhook.Enabled() {
		if err := n.webhook.SendInitialAlert(ctx, eq); err != nil {
			n.logger.Error("Webhook notification failed", "id", eq.ID, "error", err)
		} else {
			n.logger.Info("Webhook alert sent", "id", eq.ID)
		}
	}
}

// NotifyTransformError sends a dev alert when PHIVOLCS data fails to parse.
func (n *NotificationService) NotifyTransformError(ctx context.Context, field, rawValue, errMsg string) {
	message := fmt.Sprintf(
		"⚠️ *Parser Alert*\n\n"+
			"PHIVOLCS entry failed to parse.\n\n"+
			"*Field:* %s\n"+
			"*Raw value:* `%s`\n"+
			"*Error:* %s\n\n"+
			"_Action: Check if PHIVOLCS changed their HTML format._",
		field,
		rawValue,
		errMsg,
	)

	if n.telegram.Enabled() {
		if err := n.telegram.SendRawMessage(ctx, message); err != nil {
			n.logger.Error("Telegram dev alert failed", "error", err)
		}
	}

	if n.discord.Enabled() {
		if err := n.discord.SendDevAlert(ctx, field, rawValue, errMsg); err != nil {
			n.logger.Error("Discord dev alert failed", "error", err)
		}
	}
}

// NotifyEnrichment sends follow-up alerts with PHIVOLCS enrichment data.
func (n *NotificationService) NotifyEnrichment(ctx context.Context, eq models.Earthquake) {
	if n.telegram.Enabled() {
		if err := n.telegram.SendEnrichmentUpdate(ctx, eq); err != nil {
			n.logger.Error("Telegram enrichment update failed", "id", eq.ID, "error", err)
		}
	}

	if n.discord.Enabled() {
		if err := n.discord.SendEnrichmentUpdate(ctx, eq); err != nil {
			n.logger.Error("Discord enrichment update failed", "id", eq.ID, "error", err)
		}
	}

	if n.webhook.Enabled() {
		if err := n.webhook.SendEnrichmentUpdate(ctx, eq); err != nil {
			n.logger.Error("Webhook enrichment update failed", "id", eq.ID, "error", err)
		}
	}
}
