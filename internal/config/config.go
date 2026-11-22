package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Account struct {
	Backend string `json:"backend"`
	BaseURL string `json:"base_url"`
	UserID  string `json:"user_id"`
	Token   string `json:"token"`
}

type Config struct {
	Accounts []Account `json:"accounts"`
	Active   int       `json:"active"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, "citrus")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
