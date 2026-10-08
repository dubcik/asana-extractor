package main

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("ASANA_TOKEN", "test-token")
	t.Setenv("ASANA_WORKSPACE_GID", "workspace-gid")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("ASANA_REQUESTS_PER_MINUTE", "")
	t.Setenv("HTTP_TIMEOUT", "")
	t.Setenv("MAX_RETRIES", "")
	t.Setenv("OUTPUT_DIR", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, 5*time.Minute)
	}
	if cfg.AsanaRequestsPerMinute != 120 {
		t.Errorf("AsanaRequestsPerMinute = %d, want 120", cfg.AsanaRequestsPerMinute)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout = %v, want %v", cfg.HTTPTimeout, 30*time.Second)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.OutputDir != "./output" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "./output")
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("ASANA_TOKEN", "test-token")
	t.Setenv("ASANA_WORKSPACE_GID", "workspace-gid")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("ASANA_REQUESTS_PER_MINUTE", "150")
	t.Setenv("HTTP_TIMEOUT", "10s")
	t.Setenv("MAX_RETRIES", "5")
	t.Setenv("OUTPUT_DIR", "data")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.PollInterval != 30*time.Second ||
		cfg.AsanaRequestsPerMinute != 150 ||
		cfg.HTTPTimeout != 10*time.Second ||
		cfg.MaxRetries != 5 ||
		cfg.OutputDir != "data" {
		t.Errorf("loadConfig() did not apply overrides: %+v", cfg)
	}
}

func TestLoadConfigRequiresCredentials(t *testing.T) {
	t.Setenv("ASANA_TOKEN", "")
	t.Setenv("ASANA_WORKSPACE_GID", "workspace-gid")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() error = nil, want missing-token error")
	}

	t.Setenv("ASANA_TOKEN", "test-token")
	t.Setenv("ASANA_WORKSPACE_GID", "")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() error = nil, want missing-workspace error")
	}
}
