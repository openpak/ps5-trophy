package config

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad(t *testing.T) {
	if _, err := Load(env(nil)); err == nil {
		t.Fatal("missing token key accepted")
	}
	c, err := Load(env(map[string]string{"PS5_TOKEN_KEY": "0123456789abcdef0123456789abcdef", "PS5_TROPHY_SETS_DIR": "/sets"}))
	if err != nil || c.Addr != ":8080" || c.SetsDir != "/sets" || c.InternalKey != "" {
		t.Fatalf("%+v %v", c, err)
	}
}
