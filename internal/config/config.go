// Package config loads ps5-trophy configuration from the environment.
package config

import (
	"errors"

	common "github.com/openpak/ps5-common/config"
)

// Config is the shared PS5 configuration plus the trophy-specific settings.
type Config struct {
	*common.Config
	SetsDir     string // PS5_TROPHY_SETS_DIR: directory of trophy-set *.json files, optional
	InternalKey string // PS5_TROPHY_INTERNAL_KEY: enables POST /internal/unlock when set
}

// Load reads the environment through getenv (os.Getenv in production).
func Load(getenv func(string) string) (*Config, error) {
	cc, err := common.Load(getenv)
	if err != nil {
		return nil, err
	}
	if cc.TokenKey == "" {
		return nil, errors.New("PS5_TOKEN_KEY is required")
	}
	return &Config{Config: cc, SetsDir: getenv("PS5_TROPHY_SETS_DIR"), InternalKey: getenv("PS5_TROPHY_INTERNAL_KEY")}, nil
}
