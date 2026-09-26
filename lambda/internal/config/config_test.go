package config

import (
	"crypto/ed25519"
	"encoding/hex"
	"reflect"
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
	t.Setenv("QUEUE_URL", "https://sqs.example.test/queue")

	cfg, err := LoadInteraction()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(cfg.PublicKey) != key {
		t.Errorf("public key = %x, want %s", cfg.PublicKey, key)
	}
	if cfg.QueueURL != "https://sqs.example.test/queue" {
		t.Errorf("queue url = %q", cfg.QueueURL)
	}
}

// 受信側にBotトークンとsaltを渡さない。漏れる面を狭くしておく。
func TestInteractionHasNoSecrets(t *testing.T) {
	declared := map[string]bool{}
	for _, f := range reflect.VisibleFields(reflect.TypeFor[Interaction]()) {
		declared[f.Tag.Get("env")] = true
	}

	for _, key := range []string{"DISCORD_BOT_TOKEN,notEmpty", "ANONYMOUS_SALT,notEmpty"} {
		if declared[key] {
			t.Errorf("Interaction declares %q, which it does not need", key)
		}
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
			t.Setenv("QUEUE_URL", "https://sqs.example.test/queue")

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
