package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	dgo "github.com/disgoorg/disgo/discord"
)

func setupCommand(t *testing.T) dgo.Interaction {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"id": "1", "application_id": "2", "type": 2, "token": "t", "version": 1,
		"channel": map[string]any{"id": testChannelID, "type": 0},
		"member": map[string]any{
			"user": map[string]any{"id": "5", "username": "u", "discriminator": "0"},
		},
		"data": map[string]any{"id": "3", "name": "setup", "type": 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	i, err := dgo.UnmarshalInteraction(body)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestRunSetupCreatesWebhookAndPanel(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": `[]`,
		"POST /channels/4/webhooks": `{"id":"77","type":1,"token":"wh-token",` +
			`"application_id":"999","channel_id":"4","name":"匿名メッセージ"}`,
	}, nil)

	if err := runSetup(context.Background(), d, setupCommand(t)); err != nil {
		t.Fatal(err)
	}

	if s.find(http.MethodPost, "/channels/4/webhooks") == nil {
		t.Fatalf("webhook was not created; requests = %+v", s.requests)
	}

	panel := s.find(http.MethodPost, "/channels/4/messages")
	if panel == nil {
		t.Fatalf("panel was not posted; requests = %+v", s.requests)
	}

	row := panel.Body["components"].([]any)[0].(map[string]any)
	button := row["components"].([]any)[0].(map[string]any)
	if button["custom_id"] != customIDOpen {
		t.Errorf("button custom_id = %v, want %q", button["custom_id"], customIDOpen)
	}

	// 結果を実行者に伝える。
	if s.find(http.MethodPatch, "/webhooks/999/t/messages/@original") == nil {
		t.Errorf("original response was not updated; requests = %+v", s.requests)
	}
}

// 二度実行してもWebhookは増やさない。
func TestRunSetupIsIdempotent(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": ownWebhook(),
	}, nil)

	if err := runSetup(context.Background(), d, setupCommand(t)); err != nil {
		t.Fatal(err)
	}
	if s.find(http.MethodPost, "/channels/4/webhooks") != nil {
		t.Error("created a second webhook")
	}
}

// Webhookを作れない場合は、権限を確認するよう伝える。
func TestRunSetupReportsMissingPermission(t *testing.T) {
	d, s := newDeps(
		map[string]string{"GET /channels/4/webhooks": `[]`},
		map[string]int{"POST /channels/4/webhooks": http.StatusForbidden},
	)

	err := runSetup(context.Background(), d, setupCommand(t))

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v, want *UserError", err)
	}
	if !strings.Contains(userErr.Message, "ウェブフックの管理") {
		t.Errorf("message = %q, want it to mention the required permission", userErr.Message)
	}
	// Webhookが用意できていないのにパネルを置くと、押すと失敗するパネルが残る。
	if s.find(http.MethodPost, "/channels/4/messages") != nil {
		t.Error("posted the panel even though the webhook could not be created")
	}
}
