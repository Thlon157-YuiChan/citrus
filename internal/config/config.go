package config

type Config struct {
	Accounts []Account `json:"accounts"`
}

type Account struct {
	Backend string `json:"backend"`
	BaseURL string `json:"base_url"`
	UserID  string `json:"user_id"`
}

func Load() (Config, error) { return Config{}, nil }
func Save(Config) error     { return nil }
