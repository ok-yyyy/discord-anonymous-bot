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

// 投稿内容が公開チャンネルに漏れないよう、その場で返す応答は実行者にだけ見せる。
func TestMessageIsEphemeral(t *testing.T) {
	resp, err := Respond(Message("hi"))
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
	if got.Data.Flags&int(dgo.MessageFlagEphemeral) == 0 {
		t.Errorf("flags = %d, want the ephemeral flag", got.Data.Flags)
	}
}

// parseは空配列で送る必要がある。
// nullだとDiscordは未指定として扱い、メンションがそのまま飛んでしまう。
func TestMessageSuppressesMentions(t *testing.T) {
	resp, err := Respond(Message("@everyone"))
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

// embedを返す場合もメンションの抑止とephemeralが効くこと。
func TestEmbed(t *testing.T) {
	resp, err := Respond(Embed(dgo.Embed{Title: "だいめい", Description: "@everyone"}))
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Data struct {
			Content string `json:"content"`
			Flags   int    `json:"flags"`
			Embeds  []struct {
				Title string `json:"title"`
			} `json:"embeds"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}

	if len(got.Data.Embeds) != 1 || got.Data.Embeds[0].Title != "だいめい" {
		t.Errorf("embeds = %+v, want one embed titled だいめい", got.Data.Embeds)
	}
	if got.Data.Content != "" {
		t.Errorf("content = %q, want empty", got.Data.Content)
	}
	if got.Data.Flags&int(dgo.MessageFlagEphemeral) == 0 {
		t.Errorf("flags = %d, want the ephemeral flag", got.Data.Flags)
	}
	// embedの中の @everyone も飛ばさない。
	if !strings.Contains(resp.Body, `"parse":[]`) {
		t.Errorf("body = %s, want allowed_mentions.parse to be an empty array", resp.Body)
	}
}

// modalSubmit / slashCommand はackの分岐を試すためのInteraction。
func interactionOfType(t *testing.T, raw string) dgo.Interaction {
	t.Helper()

	i, err := dgo.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// MODAL_SUBMITは無言ackにする。dataを付けるとDiscordが拒否しうるのでtypeだけ送る。
func TestAckIsSilentForModalSubmit(t *testing.T) {
	i := interactionOfType(t, `{"id":"1","application_id":"2","type":5,"token":"t","version":1,
		"channel":{"id":"4","type":0},
		"member":{"user":{"id":"5","username":"u","discriminator":"0"}},
		"data":{"custom_id":"x","components":[]}}`)

	if AckShowsThinking(i) {
		t.Error("AckShowsThinking = true, want false for a modal submit")
	}

	resp, err := Respond(Ack(i))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	if got["type"] != float64(dgo.InteractionResponseTypeDeferredUpdateMessage) {
		t.Errorf("type = %v, want %d", got["type"], dgo.InteractionResponseTypeDeferredUpdateMessage)
	}
	if _, ok := got["data"]; ok {
		t.Errorf("body = %s, want no data", resp.Body)
	}
}

// スラッシュコマンドではtype 6が使えないので、ephemeralなdeferredになる。
func TestAckIsEphemeralDeferredForSlashCommand(t *testing.T) {
	i := interactionOfType(t, `{"id":"1","application_id":"2","type":2,"token":"t","version":1,
		"channel":{"id":"4","type":0},
		"member":{"user":{"id":"5","username":"u","discriminator":"0"}},
		"data":{"id":"3","name":"setup","type":1}}`)

	if !AckShowsThinking(i) {
		t.Error("AckShowsThinking = false, want true for a slash command")
	}

	resp, err := Respond(Ack(i))
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Type int `json:"type"`
		Data struct {
			Flags int `json:"flags"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != int(dgo.InteractionResponseTypeDeferredCreateMessage) {
		t.Errorf("type = %d, want %d", got.Type, dgo.InteractionResponseTypeDeferredCreateMessage)
	}
	if got.Data.Flags&int(dgo.MessageFlagEphemeral) == 0 {
		t.Errorf("flags = %d, want the ephemeral flag", got.Data.Flags)
	}
}
