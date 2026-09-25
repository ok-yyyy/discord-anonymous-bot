package config

import (
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
)

func validKey(t *testing.T) string {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(pub)
}

func TestLoadInteraction(t *testing.T) {
	key := validKey(t)
	t.Setenv("DISCORD_PUBLIC_KEY", key)

	cfg, err := LoadInteraction()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(cfg.PublicKey) != key {
		t.Errorf("public key = %x, want %s", cfg.PublicKey, key)
	}
}

func TestLoadInteractionRejectsBadKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"not hex", "not-a-hex-string"},
		{"too short", hex.EncodeToString([]byte("short"))},
		{"too long", hex.EncodeToString(make([]byte, ed25519.PublicKeySize+1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISCORD_PUBLIC_KEY", tt.key)

			if _, err := LoadInteraction(); err == nil {
				t.Error("want an error")
			}
		})
	}
}

// 不足している変数名がエラーに出ること。原因を追う手がかりになる。
func TestLoadInteractionNamesMissingVariables(t *testing.T) {
	t.Setenv("DISCORD_PUBLIC_KEY", "")

	_, err := LoadInteraction()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "DISCORD_PUBLIC_KEY") {
		t.Errorf("error = %q, want it to name the variable", err)
	}
}

// 鍵の中身をエラーに含めない。取り違えて秘匿値が入っていた場合に漏らさないため。
func TestLoadInteractionDoesNotLeakValue(t *testing.T) {
	const secret = "zzzz-looks-like-a-token-zzzz"
	t.Setenv("DISCORD_PUBLIC_KEY", secret)

	_, err := LoadInteraction()
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error = %q, want it not to contain the value", err)
	}
}
