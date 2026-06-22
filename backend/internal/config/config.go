// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	// BotToken is the Telegram Bot API token (from @BotFather).
	BotToken string
	// ChannelID identifies the source channel. Either a numeric chat id
	// (e.g. "-1001234567890") or a public username (e.g. "@mychannel").
	ChannelID string
	// PollInterval is how often to long-poll Telegram for updates.
	PollInterval time.Duration
	// HTTPAddr is the listen address for the HTTP server (e.g. ":8080").
	HTTPAddr string
	// MessagesToShow is how many recent messages to render on the display.
	MessagesToShow int
	// StorePath is the file path for persisting messages and the poll offset.
	StorePath string
}

// Load reads configuration from the environment, applying defaults for
// optional values. BotToken and ChannelID are required.
func Load() (Config, error) {
	cfg := Config{
		BotToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChannelID:      os.Getenv("CHANNEL_ID"),
		PollInterval:   getDuration("POLL_INTERVAL", 5*time.Second),
		HTTPAddr:       getString("HTTP_ADDR", ":8080"),
		MessagesToShow: getInt("MESSAGES_TO_SHOW", 5),
		StorePath:      getString("STORE_PATH", "data/store.json"),
	}

	if cfg.BotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.ChannelID == "" {
		return Config{}, fmt.Errorf("CHANNEL_ID is required")
	}
	return cfg, nil
}

func getString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
