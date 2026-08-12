package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration.
type Config struct {
	Port string
	Env  string

	// USGS polling
	USGSPollIntervalSec int
	MinMagnitude        float64

	// PHIVOLCS enrichment
	PhivolcsDelaySec       int
	PhivolcsMatchTimeMin   int
	PhivolcsMatchDistDeg   float64
	PhivolcsPrimary        bool
	PhivolcsPollIntervalSec int

	// Philippine region bounding box
	RegionMinLat float64
	RegionMaxLat float64
	RegionMinLon float64
	RegionMaxLon float64

	// Telegram
	TelegramBotToken string
	TelegramChatID   string

	// Discord
	DiscordWebhookURL string

	// Generic webhook
	WebhookURL string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "3000"),
		Env:  getEnv("ENV", "development"),

		USGSPollIntervalSec: getEnvInt("USGS_POLL_INTERVAL_SEC", 120),
		MinMagnitude:        getEnvFloat("MIN_MAGNITUDE", 2.5),

		PhivolcsDelaySec:     getEnvInt("PHIVOLCS_DELAY_SEC", 180),
		PhivolcsMatchTimeMin: getEnvInt("PHIVOLCS_MATCH_TIME_WINDOW_MIN", 15),
		PhivolcsMatchDistDeg: getEnvFloat("PHIVOLCS_MATCH_DISTANCE_DEG", 0.5),
		PhivolcsPrimary:      getEnv("PHIVOLCS_PRIMARY", "false") == "true",
		PhivolcsPollIntervalSec: getEnvInt("PHIVOLCS_POLL_INTERVAL_SEC", 300),

		RegionMinLat: getEnvFloat("REGION_MIN_LAT", 4.5),
		RegionMaxLat: getEnvFloat("REGION_MAX_LAT", 21.5),
		RegionMinLon: getEnvFloat("REGION_MIN_LON", 116.0),
		RegionMaxLon: getEnvFloat("REGION_MAX_LON", 128.0),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),

		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
		WebhookURL:        getEnv("WEBHOOK_URL", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := getEnv(key, "")
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func getEnvFloat(key string, defaultVal float64) float64 {
	val := getEnv(key, "")
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}
