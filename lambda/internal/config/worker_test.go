package config

import (
	"strings"
	"testing"
)

func TestLoadWorker(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token")
	t.Setenv("DISCORD_APPLICATION_ID", "123456789012345678")
	t.Setenv("ANONYMOUS_SALT", "salt")

	cfg, err := LoadWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "token" || cfg.AnonymousSalt != "salt" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.ApplicationID.String() != "123456789012345678" {
		t.Errorf("application id = %s", cfg.ApplicationID)
	}
}

// saltが空のまま起動すると、匿名IDが全員同じになってしまう。
func TestLoadWorkerRequiresSalt(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token")
	t.Setenv("DISCORD_APPLICATION_ID", "1")
	t.Setenv("ANONYMOUS_SALT", "")

	_, err := LoadWorker()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "ANONYMOUS_SALT") {
		t.Errorf("error = %q, want it to name ANONYMOUS_SALT", err)
	}
}
