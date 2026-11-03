package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const key = "0123456789abcdef0123456789abcdef"

func TestLoad(t *testing.T) {
	if _, err := Load(env(nil)); err == nil {
		t.Fatal("missing token key accepted")
	}
	c, err := Load(env(map[string]string{"PS5_TOKEN_KEY": key, "PS5_TROPHY_SETS_DIR": "/sets",
		"PS5_ACHIEVEMENTS_ADDR": "ach:7100", "PS5_ACHIEVEMENTS_KEY": key}))
	if err != nil || c.Addr != ":8080" || c.SetsDir != "/sets" || c.InternalKey != "" || c.DevMemory || c.AchievementsAddr != "ach:7100" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestRefusesWithoutAchievementsCore(t *testing.T) {
	_, err := Load(env(map[string]string{"PS5_TOKEN_KEY": key}))
	if err == nil || !strings.Contains(err.Error(), "PS5_ACHIEVEMENTS_ADDR") {
		t.Fatal(err)
	}
	if _, err := Load(env(map[string]string{"PS5_TOKEN_KEY": key, "PS5_ACHIEVEMENTS_ADDR": "a:1", "PS5_ACHIEVEMENTS_KEY": "short"})); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestDevMemoryNeedsNoCore(t *testing.T) {
	c, err := Load(env(map[string]string{"PS5_TOKEN_KEY": key, "PS5_DEV_MEMORY": "1"}))
	if err != nil || !c.DevMemory {
		t.Fatal(c, err)
	}
}
