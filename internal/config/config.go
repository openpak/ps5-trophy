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
	// Earned trophies live in the neutral achievements core (gRPC), never in this process.
	AchievementsAddr string // PS5_ACHIEVEMENTS_ADDR, e.g. openpak-achievements:7100; required unless PS5_DEV_MEMORY=1
	AchievementsKey  string // PS5_ACHIEVEMENTS_KEY: the core's internal key, at least 32 bytes
	DevMemory        bool   // PS5_DEV_MEMORY=1: keep earnings in memory (local development only)
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
	c := &Config{Config: cc, SetsDir: getenv("PS5_TROPHY_SETS_DIR"), InternalKey: getenv("PS5_TROPHY_INTERNAL_KEY"),
		AchievementsAddr: getenv("PS5_ACHIEVEMENTS_ADDR"), AchievementsKey: getenv("PS5_ACHIEVEMENTS_KEY"), DevMemory: common.DevMemory(getenv)}
	if !c.DevMemory {
		if c.AchievementsAddr == "" || len(c.AchievementsKey) < 32 {
			return nil, errors.New("PS5_ACHIEVEMENTS_ADDR and PS5_ACHIEVEMENTS_KEY (at least 32 bytes) are required (PS5_DEV_MEMORY=1 only for local development)")
		}
	}
	return c, nil
}
