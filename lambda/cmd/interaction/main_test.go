package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	dgo "github.com/disgoorg/disgo/discord"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/config"
)

const timestamp = "1758153600"

func newApp(t *testing.T) (*app, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &app{cfg: &config.Interaction{PublicKey: config.PublicKey(pub)}}, priv
}

func signed(priv ed25519.PrivateKey, body string) events.LambdaFunctionURLRequest {
	sig := ed25519.Sign(priv, append([]byte(timestamp), body...))
	return events.LambdaFunctionURLRequest{
		Headers: map[string]string{
			"x-signature-ed25519":   hex.EncodeToString(sig),
			"x-signature-timestamp": timestamp,
		},
		Body: body,
	}
}

// responseType はレスポンス本文からInteractionの応答種別を取り出す。
func responseType(t *testing.T, resp events.LambdaFunctionURLResponse) dgo.InteractionResponseType {
	t.Helper()

	var got struct {
		Type dgo.InteractionResponseType `json:"type"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatalf("body %q: %v", resp.Body, err)
	}
	return got.Type
}

// PINGにPONGを返せないとDiscordのエンドポイント検証に合格できない。
func TestHandlePing(t *testing.T) {
	a, priv := newApp(t)

	resp, err := a.handle(context.Background(), signed(priv, `{"id":"1","application_id":"2","type":1,"token":"t","version":1}`))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := responseType(t, resp); got != dgo.InteractionResponseTypePong {
		t.Errorf("response type = %d, want %d", got, dgo.InteractionResponseTypePong)
	}
}

// 署名が合わない場合は401。403や400だとエンドポイント検証に通らない。
func TestHandleRejectsBadSignature(t *testing.T) {
	a, priv := newApp(t)

	req := signed(priv, `{"id":"1","application_id":"2","type":1,"token":"t","version":1}`)
	req.Body = `{"id":"1","application_id":"2","type":2,"token":"t","version":1}`

	resp, err := a.handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// 署名は正しいが本文がInteractionとして読めない場合は400。
// 署名不正(401)と混同すると、エンドポイント検証の失敗原因が分からなくなる。
func TestHandleRejectsMalformedInteraction(t *testing.T) {
	a, priv := newApp(t)

	resp, err := a.handle(context.Background(), signed(priv, `{`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// 登録済みの同期コマンドはその場で本文を返す。
func TestHandleSyncCommand(t *testing.T) {
	a, priv := newApp(t)

	body := `{"id":"1","application_id":"2","type":2,"token":"t","version":1,` +
		`"data":{"id":"3","name":"ping","type":1},` +
		`"channel":{"id":"4","type":0},"user":{"id":"5","username":"u","discriminator":"0"}}`

	resp, err := a.handle(context.Background(), signed(priv, body))
	if err != nil {
		t.Fatal(err)
	}
	if got := responseType(t, resp); got != dgo.InteractionResponseTypeCreateMessage {
		t.Fatalf("response type = %d, want %d", got, dgo.InteractionResponseTypeCreateMessage)
	}

	var got struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.Content != "pong" {
		t.Errorf("content = %q, want pong", got.Data.Content)
	}
}

// 未登録のコマンドでも応答は返す。無応答だと実行者にはエラーだけが残る。
func TestHandleUnknownInteractionStillResponds(t *testing.T) {
	a, priv := newApp(t)

	body := `{"id":"1","application_id":"2","type":2,"token":"t","version":1,` +
		`"data":{"id":"3","name":"nosuchcommand","type":1},` +
		`"channel":{"id":"4","type":0},"user":{"id":"5","username":"u","discriminator":"0"}}`

	resp, err := a.handle(context.Background(), signed(priv, body))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := responseType(t, resp); got != dgo.InteractionResponseTypeCreateMessage {
		t.Errorf("response type = %d, want %d", got, dgo.InteractionResponseTypeCreateMessage)
	}
}
