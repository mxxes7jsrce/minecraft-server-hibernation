package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal temp config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 25565 {
		t.Errorf("expected default port 25565, got %d", cfg.Server.Port)
	}
	// I prefer a longer idle time before hibernation; default is 3 but I use 5 locally
	if cfg.Hibernation.TimeBeforeSleepMin != 3 {
		t.Errorf("expected default sleep time 3, got %d", cfg.Hibernation.TimeBeforeSleepMin)
	}
}

func TestLoad_NonExistentFile_ReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := DefaultConfig()
	if cfg.Server.Port != def.Server.Port {
		t.Errorf("expected default port, got %d", cfg.Server.Port)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	raw := map[string]any{
		"server": map[string]any{
			"host": "0.0.0.0",
			"port": 25566,
			"motd": "Custom MOTD",
		},
		"hibernation": map[string]any{
			"timeBeforeSleepMin": 5,
		},
	}
	path := writeTempConfig(t, raw)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 25566 {
		t.Errorf("expected port 25566, got %d", cfg.Server.Port)
	}
	if cfg.Hibernation.TimeBeforeSleepMin != 5 {
		t.Errorf("expected sleep 5, got %d", cfg.Hibernation.TimeBeforeSleepMin)
	}
	// Fields not in file should retain defaults
	if cfg.Log.Level != "info" {
		t.Errorf("expected default log level 'info', got %q", cfg.Log.Level)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	raw := map[string]any{
		"server": map[string]any{"port": 99999},
	}
	path := writeTempConfig(t, raw)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid port, got nil")
	}
}

// Also test the lower port boundary since valid ports are 1-65535
func TestLoad_InvalidPortZero(t *testing.T) {
	raw := map[string]any{
		"server": map[string]any{"port": 0},
	}
	path := writeTempConfig(t, raw)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for port 0, got nil")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
