package discord

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	dgo "github.com/disgoorg/disgo/discord"
)

// DiscordのエンドポイントはPONGが {"type":1} であることを前提に検証する。
func TestPongSerialization(t *testing.T) {
	resp, err := Respond(Pong())
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Headers["Content-Type"]; got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	if got["type"] != float64(dgo.InteractionResponseTypePong) {
		t.Errorf("body = %s, want type %d", resp.Body, dgo.InteractionResponseTypePong)
	}
}

func TestMessageEphemeralFlag(t *testing.T) {
	tests := []struct {
		name      string
		ephemeral bool
		wantFlag  bool
	}{
		{"ephemeral", true, true},
		{"public", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := Respond(Message("hi", tt.ephemeral))
			if err != nil {
				t.Fatal(err)
			}

			var got struct {
				Data struct {
					Content string `json:"content"`
					Flags   int    `json:"flags"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
				t.Fatal(err)
			}

			if got.Data.Content != "hi" {
				t.Errorf("content = %q, want hi", got.Data.Content)
			}
			if hasFlag := got.Data.Flags&int(dgo.MessageFlagEphemeral) != 0; hasFlag != tt.wantFlag {
				t.Errorf("ephemeral flag = %v, want %v (flags = %d)", hasFlag, tt.wantFlag, got.Data.Flags)
			}
		})
	}
}

// parseは空配列で送る必要がある。nullだとDiscordは未指定として扱い、
// メンションがそのまま飛んでしまう。
func TestMessageSuppressesMentions(t *testing.T) {
	resp, err := Respond(Message("@everyone", false))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(resp.Body, `"parse":[]`) {
		t.Errorf("body = %s, want allowed_mentions.parse to be an empty array", resp.Body)
	}
}

func TestStatus(t *testing.T) {
	resp := Status(http.StatusUnauthorized)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if resp.Body != "" {
		t.Errorf("body = %q, want empty", resp.Body)
	}
}
