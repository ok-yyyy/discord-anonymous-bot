package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/queue"
)

const testAppID = snowflake.ID(999)

// stub はDiscord APIの代わりに応答し、受け取ったパスを記録する。
type stub struct {
	responses map[string]string
	paths     []string
}

func (s *stub) RoundTrip(r *http.Request) (*http.Response, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v10")
	s.paths = append(s.paths, r.Method+" "+path)

	payload, ok := s.responses[r.Method+" "+path]
	if !ok {
		payload = "{}"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(payload)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    r,
	}, nil
}

func (s *stub) called(entry string) bool {
	for _, p := range s.paths {
		if p == entry {
			return true
		}
	}
	return false
}

func newApp(responses map[string]string) (*app, *stub) {
	s := &stub{responses: responses}

	return &app{deps: &handler.Deps{
		Rest:          discord.NewRest("token", rest.WithHTTPClient(&http.Client{Transport: s})),
		ApplicationID: testAppID,
		Salt:          "test-salt",
		Now:           time.Now,
	}}, s
}

// record はSQSのメッセージを組み立てる。
func record(t *testing.T, receivedAt time.Time, interaction string) events.SQSMessage {
	t.Helper()

	body, err := json.Marshal(queue.Message{
		ReceivedAt:  receivedAt,
		Interaction: json.RawMessage(interaction),
	})
	if err != nil {
		t.Fatal(err)
	}
	return events.SQSMessage{Body: string(body)}
}

// modalSubmitJSON は匿名投稿のInteraction。
func modalSubmitJSON() string {
	return `{"id":"1","application_id":"2","type":5,"token":"t","version":1,
		"channel":{"id":"4","type":0},
		"member":{"user":{"id":"5","username":"u","discriminator":"0"}},
		"data":{"custom_id":"anon:submit","components":[
			{"type":18,"component":{"type":4,"custom_id":"message","value":"こんにちは"}}]}}`
}

func TestProcessPostsAnonymously(t *testing.T) {
	a, s := newApp(map[string]string{
		"GET /channels/4/webhooks": `[{"id":"77","type":1,"token":"wh-token",` +
			`"application_id":"999","channel_id":"4","name":"匿名メッセージ"}]`,
	})

	err := a.process(context.Background(), record(t, time.Now(), modalSubmitJSON()))
	if err != nil {
		t.Fatal(err)
	}
	if !s.called("POST /webhooks/77/wh-token") {
		t.Errorf("webhook was not executed; calls = %v", s.paths)
	}
}

// tokenが切れていたらfollowupも送れない。処理せず捨てる。
func TestProcessDiscardsExpiredToken(t *testing.T) {
	a, s := newApp(nil)

	old := time.Now().Add(-tokenLifetime - time.Minute)
	if err := a.process(context.Background(), record(t, old, modalSubmitJSON())); err != nil {
		t.Fatalf("err = %v, want nil so the message is not retried", err)
	}
	if len(s.paths) != 0 {
		t.Errorf("called Discord for an expired interaction: %v", s.paths)
	}
}

// 想定内の失敗は実行者に伝えて終える。再試行しても直らない。
func TestProcessReportsUserErrorWithoutRetrying(t *testing.T) {
	a, s := newApp(map[string]string{
		// Webhookが無いのでsetupのやり直しを促す経路に入る。
		"GET /channels/4/webhooks": `[]`,
	})

	if err := a.process(context.Background(), record(t, time.Now(), modalSubmitJSON())); err != nil {
		t.Fatalf("err = %v, want nil so the message is not sent to the DLQ", err)
	}
	// 無言ackなのでfollowupで伝える。@originalはパネルを指すため編集しない。
	if !s.called("POST /webhooks/999/t") {
		t.Errorf("the user was not notified; calls = %v", s.paths)
	}
	if s.called("PATCH /webhooks/999/t/messages/@original") {
		t.Error("edited the panel message instead of sending a followup")
	}
}

// 登録されていないInteractionはDLQに送る。デプロイのずれを検知するため。
func TestProcessFailsOnUnknownInteraction(t *testing.T) {
	a, _ := newApp(nil)

	unknown := `{"id":"1","application_id":"2","type":5,"token":"t","version":1,
		"channel":{"id":"4","type":0},
		"member":{"user":{"id":"5","username":"u","discriminator":"0"}},
		"data":{"custom_id":"anon:nosuch","components":[]}}`

	if err := a.process(context.Background(), record(t, time.Now(), unknown)); err == nil {
		t.Error("want an error so the message goes to the DLQ")
	}
}

func TestProcessFailsOnMalformedMessage(t *testing.T) {
	a, _ := newApp(nil)

	if err := a.process(context.Background(), events.SQSMessage{Body: "{"}); err == nil {
		t.Error("want an error")
	}
}
