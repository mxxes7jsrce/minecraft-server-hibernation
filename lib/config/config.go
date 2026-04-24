package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all configuration for minecraft-server-hibernation
type Config struct {
	Server   ServerConfig   `json:"server"`
	Hibernation HibernationConfig `json:"hibernation"`
	Log      LogConfig      `json:"log"`
}

// ServerConfig holds Minecraft server connection settings
type ServerConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Protocol       int    `json:"protocol"`
	MaxPlayers     int    `json:"maxPlayers"`
	Motd           string `json:"motd"`
	StartCommand   string `json:"startCommand"`
	StopCommand    string `json:"stopCommand"`
}

// HibernationConfig holds hibernation/sleep settings
type HibernationConfig struct {
	TimeBeforeSleepMin int  `json:"timeBeforeSleepMin"`
	NotifyEnabled      bool `json:"notifyEnabled"`
	NotifyMessage      string `json:"notifyMessage"`
}

// LogConfig holds logging settings
type LogConfig struct {
	Level  string `json:"level"`
	ToFile bool   `json:"toFile"`
	Path   string `json:"path"`
}

// DefaultConfig returns a Config populated with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "127.0.0.1",
			Port:         25565,
			Version:      "1.20.1",
			Protocol:     763,
			MaxPlayers:   20,
			// Customized MOTD to make it clearer the server will start on join
			Motd:         "Hibernating - join to wake the server!",
			StartCommand: "",
			StopCommand:  "",
		},
		Hibernation: HibernationConfig{
			// Bumped to 5 minutes to avoid thrashing on my low-spec home server
			TimeBeforeSleepMin: 5,
			NotifyEnabled:      true,
			NotifyMessage:      "Server is starting, please wait...",
		},
		Log: LogConfig{
			Level:  "info",
			ToFile: false,
			Path:   "msh.log",
		},
	}
}

// Load reads and parses a JSON config file from the given path.
// Missing fields fall back to defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("config: reading file %q: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parsing file %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

// validate performs basic sanity checks on the loaded configuration.
func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range (1-65535)", c.Server.Port)
	}
	if c.Hibernation.TimeBeforeSleepMin < 0 {
		return fmt.Errorf("hibernation.timeBeforeSleepMin must be >= 0")
	}
	return nil
}
