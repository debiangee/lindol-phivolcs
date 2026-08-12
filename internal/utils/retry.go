// Package utils provides shared utilities.
package utils

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   2 * time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// WithRetry executes a function with exponential backoff retry.
func WithRetry(ctx context.Context, cfg RetryConfig, logger *slog.Logger, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}

			logger.Warn("Operation failed, retrying",
				"operation", operation,
				"attempt", attempt+1,
				"max_attempts", cfg.MaxAttempts,
				"next_retry_in", delay,
				"error", lastErr,
			)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, cfg.MaxAttempts, lastErr)
}
