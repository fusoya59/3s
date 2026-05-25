package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all configuration for 3s.
type Config struct {
	Locale             string                    `json:"locale"`
	Safesearch         int                       `json:"safesearch"`
	UserAgent          string                    `json:"user_agent"`
	SearchTimeout      int                       `json:"search_timeout"`
	ScrapeTimeout      int                       `json:"scrape_timeout"`
	ContentMinChars    int                       `json:"content_min_chars"`
	ContentPollTimeout int                       `json:"content_poll_timeout"`
	CachePath          string                    `json:"cache_path"`
	CacheTTL           int                       `json:"cache_ttl"`
	BrowserBinPath     string                    `json:"browser_bin_path"`
	EngineConfig       map[string]map[string]any `json:"engine_config"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	cacheDir, _ := os.UserCacheDir()
	defaultCache := filepath.Join(cacheDir, "3s", "cache.db")

	return &Config{
		Locale:             "en-US",
		Safesearch:         0,
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64; rv:135.0) Gecko/20100101 Firefox/135.0",
		SearchTimeout:      15,
		ScrapeTimeout:      30,
		ContentMinChars:    500,
		ContentPollTimeout: 5,
		CachePath:          defaultCache,
		CacheTTL:           300,
		BrowserBinPath:     "",
		EngineConfig: map[string]map[string]any{
			"brave":      {},
			"duckduckgo": {},
		},
	}
}

// Load reads a JSON config file. Returns defaults if path is empty.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	// Expand ~ in path
	path, err := expandPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "3s", "config.json")
}

// Validate checks config values and returns any issues.
func (c *Config) Validate() error {
	if c.SearchTimeout < 5 || c.SearchTimeout > 60 {
		return fmt.Errorf("search_timeout must be between 5 and 60")
	}
	if c.ScrapeTimeout < 10 || c.ScrapeTimeout > 120 {
		return fmt.Errorf("scrape_timeout must be between 10 and 120")
	}
	if c.ContentMinChars < 100 || c.ContentMinChars > 100000 {
		return fmt.Errorf("content_min_chars must be between 100 and 100000")
	}
	if c.ContentPollTimeout < 1 || c.ContentPollTimeout > 30 {
		return fmt.Errorf("content_poll_timeout must be between 1 and 30")
	}
	if c.CacheTTL < 0 {
		return fmt.Errorf("cache_ttl must be >= 0")
	}
	if c.Safesearch < 0 || c.Safesearch > 2 {
		return fmt.Errorf("safesearch must be 0, 1, or 2")
	}
	if c.Locale == "" {
		c.Locale = "en-US"
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultConfig().UserAgent
	}
	if c.EngineConfig == nil {
		c.EngineConfig = make(map[string]map[string]any)
	}

	return nil
}

// ExpandPath expands ~ in file paths within the config.
func (c *Config) ExpandPath() error {
	var err error
	c.CachePath, err = expandPath(c.CachePath)
	if err != nil {
		return err
	}
	c.BrowserBinPath, err = expandPath(c.BrowserBinPath)
	return err
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home dir: %w", err)
		}
		return filepath.Join(home, p[1:]), nil
	}
	return p, nil
}
