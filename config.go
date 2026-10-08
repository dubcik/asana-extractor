package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AsanaToken             string
	AsanaWorkspaceGID      string
	PollInterval           time.Duration
	AsanaRequestsPerMinute int
	HTTPTimeout            time.Duration
	MaxRetries             int
	OutputDir              string
}

func loadConfig() (Config, error) {
	cfg := Config{
		AsanaToken:        strings.TrimSpace(os.Getenv("ASANA_TOKEN")),
		AsanaWorkspaceGID: strings.TrimSpace(os.Getenv("ASANA_WORKSPACE_GID")),
		OutputDir:         strings.TrimSpace(os.Getenv("OUTPUT_DIR")),
	}

	if cfg.AsanaToken == "" {
		return Config{}, fmt.Errorf("ASANA_TOKEN is required")
	}
	if cfg.AsanaWorkspaceGID == "" {
		return Config{}, fmt.Errorf("ASANA_WORKSPACE_GID is required")
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./output"
	}

	var err error
	if cfg.PollInterval, err = durationFromEnv("POLL_INTERVAL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.AsanaRequestsPerMinute, err = intFromEnv("ASANA_REQUESTS_PER_MINUTE", 120); err != nil {
		return Config{}, err
	}
	if cfg.HTTPTimeout, err = durationFromEnv("HTTP_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MaxRetries, err = intFromEnv("MAX_RETRIES", 3); err != nil {
		return Config{}, err
	}

	if cfg.PollInterval <= 0 {
		return Config{}, fmt.Errorf("POLL_INTERVAL must be greater than zero")
	}
	if cfg.AsanaRequestsPerMinute <= 0 {
		return Config{}, fmt.Errorf("ASANA_REQUESTS_PER_MINUTE must be greater than zero")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_TIMEOUT must be greater than zero")
	}
	if cfg.MaxRetries < 0 {
		return Config{}, fmt.Errorf("MAX_RETRIES must not be negative")
	}

	return cfg, nil
}

func durationFromEnv(name string, defaultValue time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	return duration, nil
}

func intFromEnv(name string, defaultValue int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return parsed, nil
}
