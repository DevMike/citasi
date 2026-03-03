package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Province     string
	OficinaText  string // visible text of oficina to select (empty = keep default "Cualquier oficina")
	TramiteText  string
	DocType      string // "nie", "passport", or "dni"
	DocNumber    string
	FullName     string
	Country      string
	Phone        string
	Email        string

	TelegramToken  string
	TelegramChatID string

	CheckInterval    int // seconds between checks
	CheckJitter      int // max random jitter in seconds
	MaxCycles        int // 0 = unlimited
	PageTimeout      int // seconds for page operations
	SaveScreenshots  bool
	ScreenshotDir    string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // ignore error if .env doesn't exist

	cfg := &Config{
		Province:       os.Getenv("PROVINCE"),
		OficinaText:    os.Getenv("OFICINA_TEXT"),
		TramiteText:    os.Getenv("TRAMITE_TEXT"),
		DocType:        os.Getenv("DOC_TYPE"),
		DocNumber:      os.Getenv("DOC_NUMBER"),
		FullName:       os.Getenv("FULL_NAME"),
		Country:        os.Getenv("COUNTRY"),
		Phone:          os.Getenv("PHONE"),
		Email:          os.Getenv("EMAIL"),
		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		ScreenshotDir:  os.Getenv("SCREENSHOT_DIR"),
	}

	// Defaults
	if cfg.ScreenshotDir == "" {
		cfg.ScreenshotDir = "screenshots"
	}

	cfg.CheckInterval = envInt("CHECK_INTERVAL", 60)
	cfg.CheckJitter = envInt("CHECK_JITTER", 15)
	cfg.MaxCycles = envInt("MAX_CYCLES", 0)
	cfg.PageTimeout = envInt("PAGE_TIMEOUT", 30)
	cfg.SaveScreenshots = envBool("SAVE_SCREENSHOTS", true)

	// Validate required fields
	required := map[string]string{
		"PROVINCE":     cfg.Province,
		"TRAMITE_TEXT": cfg.TramiteText,
		"DOC_TYPE":     cfg.DocType,
		"DOC_NUMBER":   cfg.DocNumber,
		"FULL_NAME":    cfg.FullName,
		"COUNTRY":      cfg.Country,
	}
	for name, val := range required {
		if val == "" {
			return nil, fmt.Errorf("required env var %s is not set", name)
		}
	}

	validDocTypes := map[string]bool{"nie": true, "passport": true, "dni": true}
	if !validDocTypes[cfg.DocType] {
		return nil, fmt.Errorf("DOC_TYPE must be one of: nie, passport, dni (got %q)", cfg.DocType)
	}

	return cfg, nil
}

func MustLoadConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
