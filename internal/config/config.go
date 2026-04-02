package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Endpoint    string `json:"endpoint"`
	Org         string `json:"org"`
	Project     string `json:"project"`
	Token       string `json:"token"`
	Concurrency int    `json:"concurrency"`
}

func defaultConfig() Config {
	return Config{Concurrency: 4}
}

// normalizeEndpoint ensures the endpoint always ends with /api/v2,
// regardless of whether the user supplied it or not.
func normalizeEndpoint(e string) string {
	e = strings.TrimRight(e, "/")
	if strings.HasSuffix(e, "/api/v2") {
		return e
	}
	return e + "/api/v2"
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kh"), nil
}

func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

func Load() (Config, error) {
	cfg := defaultConfig()
	p, err := Path()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// LoadWithEnv loads config from disk and applies environment variable overrides
func LoadWithEnv() (Config, error) {
	cfg, err := Load()
	if err != nil {
		return cfg, err
	}
	// Apply environment variable overrides
	if v := os.Getenv("KH_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if cfg.Endpoint != "" {
		cfg.Endpoint = normalizeEndpoint(cfg.Endpoint)
	}
	if v := os.Getenv("KH_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("KH_ORG"); v != "" {
		cfg.Org = v
	}
	if v := os.Getenv("KH_PROJECT"); v != "" {
		cfg.Project = v
	}
	if v := os.Getenv("KH_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency = n
		}
	}
	return cfg, nil
}

// Helpers with precedence: flag > env > config default
func FromEnvOr(cfg Config, key string, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	switch key {
	case "KH_ENDPOINT":
		if cfg.Endpoint != "" {
			return cfg.Endpoint
		}
	case "KH_ORG":
		if cfg.Org != "" {
			return cfg.Org
		}
	case "KH_PROJECT":
		if cfg.Project != "" {
			return cfg.Project
		}
	case "KH_TOKEN":
		if cfg.Token != "" {
			return cfg.Token
		}
	}
	return def
}

func FromEnvOrInt(cfg Config, key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	switch key {
	case "KH_CONCURRENCY":
		if cfg.Concurrency > 0 {
			return cfg.Concurrency
		}
	}
	return def
}

func Get(cfg Config, field string) (string, error) {
	switch field {
	case "endpoint":
		return cfg.Endpoint, nil
	case "org":
		return cfg.Org, nil
	case "project":
		return cfg.Project, nil
	case "token":
		if cfg.Token == "" {
			return "", fmt.Errorf("no token configured")
		}
		return cfg.Token, nil
	case "concurrency":
		return fmt.Sprint(cfg.Concurrency), nil
	default:
		return "", fmt.Errorf("unknown key: %s", field)
	}
}

func Set(cfg *Config, field, value string) error {
	switch field {
	case "endpoint":
		cfg.Endpoint = value
	case "org":
		cfg.Org = value
	case "project":
		cfg.Project = value
	case "token":
		cfg.Token = value
	case "concurrency":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Concurrency = n
	default:
		return fmt.Errorf("unknown key: %s", field)
	}
	return nil
}
