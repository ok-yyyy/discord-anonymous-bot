package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

const (
	testAppID     = snowflake.ID(999)
	testChannelID = "4"
)

// recordedRequest はstubが受け取ったリクエスト。
type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// stub はDiscord APIの代わりに応答する。
// responsesのキーは "METHOD /path"、値は返すJSON。statusesで状態コードを変えられる。
type stub struct {
	responses map[string]string
	statuses  map[string]int
	requests  []recordedRequest
}

func (s *stub) RoundTrip(r *http.Request) (*http.Response, error) {
	var body map[string]any
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v10")
	key := r.Method + " " + path
	s.requests = append(s.requests, recordedRequest{Method: r.Method, Path: path, Body: body})

	status := http.StatusOK
	if s.statuses != nil {
		if code, ok := s.statuses[key]; ok {
			status = code
		}
	}

	payload, ok := s.responses[key]
	if !ok {
		payload = "{}"
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(payload)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    r,
	}, nil
}

// indexOf はリクエストが何番目に送られたかを返す。送られていなければ-1。
func (s *stub) indexOf(method, path string) int {
	for i := range s.requests {
		if s.requests[i].Method == method && s.requests[i].Path == path {
			return i
		}
	}
	return -1
}

func (s *stub) find(method, path string) *recordedRequest {
	for i := range s.requests {
		if s.requests[i].Method == method && s.requests[i].Path == path {
			return &s.requests[i]
		}
	}
	return nil
}

func newDeps(responses map[string]string, statuses map[string]int) (*Deps, *stub) {
	s := &stub{responses: responses, statuses: statuses}

	return &Deps{
		Rest:          discord.NewRest("token", rest.WithHTTPClient(&http.Client{Transport: s})),
		ApplicationID: testAppID,
		Salt:          "test-salt",
		Now:           func() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) },
	}, s
}

// ownWebhook はこのアプリが作ったWebhookのJSON。
func ownWebhook() string {
	return `[{"id":"77","type":1,"token":"wh-token","application_id":"999","channel_id":"4","name":"匿名メッセージ"}]`
}

// panelMessageID は投稿パネルのメッセージID。
const panelMessageID = "555"

func modalSubmit(t *testing.T, content string) dgo.Interaction {
	t.Helper()
	return modalSubmitFrom(t, content, true)
}

// modalSubmitFrom はモーダル送信のInteractionを組み立てる。
// fromPanelがtrueなら、パネルのボタンから開かれた場合と同じくmessageを含める。
func modalSubmitFrom(t *testing.T, content string, fromPanel bool) dgo.Interaction {
	t.Helper()

	payload := map[string]any{
		"id": "1", "application_id": "2", "type": 5, "token": "t", "version": 1,
		"channel": map[string]any{"id": testChannelID, "type": 0},
		"member": map[string]any{
			"user": map[string]any{"id": "5", "username": "u", "discriminator": "0"},
		},
		"data": map[string]any{
			"custom_id": customIDSubmit,
			"components": []any{
				map[string]any{
					"type": 18,
					"component": map[string]any{
						"type": 4, "custom_id": fieldMessage, "value": content,
					},
				},
			},
		},
	}
	if fromPanel {
		payload["message"] = map[string]any{
			"id": panelMessageID, "channel_id": testChannelID, "type": 0,
			"content": "", "timestamp": "2026-09-18T12:00:00Z",
			"author": map[string]any{"id": "999", "username": "bot", "discriminator": "0"},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	i, err := dgo.UnmarshalInteraction(body)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// モーダルはdeferredにできないので、同期でtype 9を返す。
func TestOpenModalReturnsModal(t *testing.T) {
	resp, err := openModal.Handle(nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Type != dgo.InteractionResponseTypeModal {
		t.Fatalf("type = %d, want %d", resp.Type, dgo.InteractionResponseTypeModal)
	}

	modal, ok := resp.Data.(dgo.ModalCreate)
	if !ok {
		t.Fatalf("data = %T, want ModalCreate", resp.Data)
	}
	if modal.CustomID != customIDSubmit {
		t.Errorf("custom_id = %q, want %q", modal.CustomID, customIDSubmit)
	}

	// 入力段階で上限を効かせないと、送信してから長すぎると分かることになる。
	label, ok := modal.Components[0].(dgo.LabelComponent)
	if !ok {
		t.Fatalf("component = %T, want LabelComponent", modal.Components[0])
	}
	input, ok := label.Component.(dgo.TextInputComponent)
	if !ok {
		t.Fatalf("label component = %T, want TextInputComponent", label.Component)
	}
	if input.MaxLength != maxMessageLength {
		t.Errorf("max_length = %d, want %d", input.MaxLength, maxMessageLength)
	}
	if input.Style != dgo.TextInputStyleParagraph {
		t.Errorf("style = %d, want paragraph", input.Style)
	}
	if !input.Required {
		t.Error("input is not required")
	}
}

// モーダルの入力欄にはラベルが必要。TextInput単体では持てないのでLabelで包む。
func TestOpenModalUsesLabelComponent(t *testing.T) {
	resp, _ := openModal.Handle(nil)
	modal := resp.Data.(dgo.ModalCreate)

	label := modal.Components[0].(dgo.LabelComponent)
	if label.Label == "" {
		t.Error("label is empty")
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"ok", "こんにちは", false},
		{"empty", "", true},
		{"whitespace only", "   \n  ", true},
		{"at the limit", strings.Repeat("あ", maxMessageLength), false},
		{"too long", strings.Repeat("あ", maxMessageLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMessage(modalSubmit(t, tt.content))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			// 検査エラーはそのまま実行者に見せるのでUserErrorであること。
			if err != nil {
				var userErr *UserError
				if !errors.As(err, &userErr) {
					t.Errorf("err = %v, want *UserError", err)
				}
			}
		})
	}
}

func TestRunPostUsesAnonymousIdentity(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": ownWebhook(),
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatal(err)
	}

	req := s.find(http.MethodPost, "/webhooks/77/wh-token")
	if req == nil {
		t.Fatalf("webhook was not executed; requests = %+v", s.requests)
	}
	if req.Body["content"] != "こんにちは" {
		t.Errorf("content = %v", req.Body["content"])
	}
	// 投稿者本人ではなく、導出した匿名の名前で投稿する。
	if name, _ := req.Body["username"].(string); name == "" || name == "u" {
		t.Errorf("username = %q, want an anonymous name", name)
	}
	if url, _ := req.Body["avatar_url"].(string); !strings.HasPrefix(url, "https://api.dicebear.com/") {
		t.Errorf("avatar_url = %q", url)
	}

	// 匿名投稿から@everyoneが飛ばないようにする。
	mentions, ok := req.Body["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("allowed_mentions missing: %v", req.Body)
	}
	if parse, ok := mentions["parse"].([]any); !ok || len(parse) != 0 {
		t.Errorf("allowed_mentions.parse = %v, want an empty array", mentions["parse"])
	}

	// 成功時は何も伝えない。投稿とパネルの張り替えで結果が分かる。
	if s.find(http.MethodPatch, "/webhooks/999/t/messages/@original") != nil {
		t.Error("updated the original response even though the ack is silent")
	}
	if s.find(http.MethodPost, "/webhooks/999/t") != nil {
		t.Error("sent a followup on success; nothing needs to be said")
	}
}

// MODAL_SUBMITは無言ackになる。@originalはモーダルを開いたパネルを指すので、
// 結果はfollowupで伝えなければパネルが書き換わる。
func TestModalSubmitAcksSilently(t *testing.T) {
	if discord.AckShowsThinking(modalSubmit(t, "x")) {
		t.Error("a modal submit should be acknowledged silently")
	}
}

// 指定漏れでメンションが飛ばないよう、クライアントの既定を空にしてある。
func TestPanelIsPostedWithoutMentions(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": ownWebhook(),
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatal(err)
	}

	panel := s.find(http.MethodPost, "/channels/4/messages")
	if panel == nil {
		t.Fatalf("panel was not posted; requests = %+v", s.requests)
	}

	mentions, ok := panel.Body["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("allowed_mentions missing: %v", panel.Body)
	}
	if parse, ok := mentions["parse"].([]any); !ok || len(parse) != 0 {
		t.Errorf("allowed_mentions.parse = %v, want an empty array", mentions["parse"])
	}
}

// 他アプリが作ったWebhookは使わない。
func TestRunPostIgnoresForeignWebhook(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": `[{"id":"88","type":1,"token":"t","application_id":"123","channel_id":"4","name":"other"}]`,
	}, nil)

	err := runPost(context.Background(), d, modalSubmit(t, "こんにちは"))

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v, want *UserError telling the user to run setup again", err)
	}
	if !strings.Contains(userErr.Message, "setup") {
		t.Errorf("message = %q, want it to mention setup", userErr.Message)
	}
	if s.find(http.MethodPost, "/webhooks/88/t") != nil {
		t.Error("posted through a webhook owned by another application")
	}
}

// Webhookが無い場合も勝手に作り直さない。権限の無いチャンネルに投稿しうるため。
func TestRunPostDoesNotRecreateWebhook(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks": `[]`,
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err == nil {
		t.Fatal("want an error")
	}
	if s.find(http.MethodPost, "/channels/4/webhooks") != nil {
		t.Error("recreated the webhook")
	}
}

// Webhookが消えていた場合はsetupのやり直しを促す。
func TestRunPostReportsDeletedWebhook(t *testing.T) {
	d, _ := newDeps(
		map[string]string{"GET /channels/4/webhooks": ownWebhook()},
		map[string]int{"POST /webhooks/77/wh-token": http.StatusNotFound},
	)

	err := runPost(context.Background(), d, modalSubmit(t, "こんにちは"))

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v, want *UserError", err)
	}
	if !strings.Contains(userErr.Message, "setup") {
		t.Errorf("message = %q, want it to mention setup", userErr.Message)
	}
}

// 投稿に失敗したことが本文と一緒にログへ出ないこと。
func TestRunPostErrorDoesNotLeakContent(t *testing.T) {
	const secret = "ひみつのないよう"

	d, _ := newDeps(
		map[string]string{"GET /channels/4/webhooks": ownWebhook()},
		map[string]int{"POST /webhooks/77/wh-token": http.StatusInternalServerError},
	)

	err := runPost(context.Background(), d, modalSubmit(t, secret))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error = %q, want it not to contain the message body", err)
	}
}

// panelMsg はBot自身が投稿したパネルのJSON。
func panelMsg(id string) string {
	return `{"id":"` + id + `","channel_id":"4","type":0,"content":"",` +
		`"timestamp":"2026-09-18T12:00:00Z",` +
		`"author":{"id":"999","username":"bot","discriminator":"0"}}`
}

// webhookMsg は匿名投稿のJSON。webhook_idが入る。
func webhookMsg(id string) string {
	return `{"id":"` + id + `","channel_id":"4","type":0,"content":"ひみつ",` +
		`"timestamp":"2026-09-18T12:00:00Z","webhook_id":"77",` +
		`"author":{"id":"77","username":"しずかなうさぎ","discriminator":"0"}}`
}

// userMsg は他の利用者のメッセージのJSON。
func userMsg(id string) string {
	return `{"id":"` + id + `","channel_id":"4","type":0,"content":"やあ",` +
		`"timestamp":"2026-09-18T12:00:00Z",` +
		`"author":{"id":"123","username":"someone","discriminator":"0"}}`
}

func history(msgs ...string) string {
	return "[" + strings.Join(msgs, ",") + "]"
}

// 投稿後はパネルを作り直し、古いパネルをまとめて消して1枚に収束させる。
func TestRunPostRefreshesPanel(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks":  ownWebhook(),
		"POST /channels/4/messages": panelMsg("900"),
		// 同じパネルから複数人がモーダルを開くと、古いパネルが複数残る。
		"GET /channels/4/messages": history(panelMsg("900"), panelMsg("801"), panelMsg("802")),
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatal(err)
	}

	created := s.indexOf(http.MethodPost, "/channels/4/messages")
	if created < 0 {
		t.Fatalf("new panel was not posted; requests = %+v", s.requests)
	}
	// 先に消すと、作成に失敗したときにパネルが1つも無い状態になる。
	for _, id := range []string{"801", "802"} {
		deleted := s.indexOf(http.MethodDelete, "/channels/4/messages/"+id)
		if deleted < 0 {
			t.Errorf("stale panel %s was not deleted; requests = %+v", id, s.requests)
			continue
		}
		if created > deleted {
			t.Errorf("deleted stale panel %s before creating the new one", id)
		}
	}
	// 今作ったパネルは消さない。
	if s.indexOf(http.MethodDelete, "/channels/4/messages/900") >= 0 {
		t.Error("deleted the panel it had just created")
	}
	// 匿名投稿より後に置かないと、パネルが一番下に来ない。
	if posted := s.indexOf(http.MethodPost, "/webhooks/77/wh-token"); posted > created {
		t.Error("posted the panel before the anonymous message")
	}
}

// 匿名メッセージはWebhook経由なので、掃除の対象にしてはいけない。
func TestRunPostNeverDeletesAnonymousMessages(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks":  ownWebhook(),
		"POST /channels/4/messages": panelMsg("900"),
		"GET /channels/4/messages": history(
			panelMsg("900"), webhookMsg("701"), webhookMsg("702"), userMsg("601"), panelMsg("801")),
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"701", "702", "601"} {
		if s.indexOf(http.MethodDelete, "/channels/4/messages/"+id) >= 0 {
			t.Errorf("deleted message %s, which is not a panel", id)
		}
	}
	if s.indexOf(http.MethodDelete, "/channels/4/messages/801") < 0 {
		t.Error("did not delete the stale panel")
	}
}

// 新パネルを作れなかったときは、古いパネルを消さずに残す。
func TestRunPostKeepsOldPanelWhenCreateFails(t *testing.T) {
	d, s := newDeps(
		map[string]string{
			"GET /channels/4/webhooks": ownWebhook(),
			"GET /channels/4/messages": history(panelMsg("801")),
		},
		map[string]int{"POST /channels/4/messages": http.StatusForbidden},
	)

	// 投稿自体は成功しているので、エラーにはしない。
	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatalf("err = %v, want nil so the message is not retried", err)
	}
	if s.indexOf(http.MethodDelete, "/channels/4/messages/801") >= 0 {
		t.Error("deleted the old panel even though the new one could not be posted")
	}
}

// 掃除に失敗しても、投稿は成功として扱う。再実行すると二重投稿になる。
func TestRunPostSucceedsWhenPanelCleanupFails(t *testing.T) {
	d, _ := newDeps(
		map[string]string{
			"GET /channels/4/webhooks":  ownWebhook(),
			"POST /channels/4/messages": panelMsg("900"),
			"GET /channels/4/messages":  history(panelMsg("900"), panelMsg("801")),
		},
		map[string]int{"DELETE /channels/4/messages/801": http.StatusNotFound},
	)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

// 履歴が空のときは何も消さない。
// 「メッセージ履歴を読む」権限が無い場合、Discordはエラーではなく空を返す。
func TestRunPostDeletesNothingWhenHistoryIsEmpty(t *testing.T) {
	d, s := newDeps(map[string]string{
		"GET /channels/4/webhooks":  ownWebhook(),
		"POST /channels/4/messages": panelMsg("900"),
		"GET /channels/4/messages":  `[]`,
	}, nil)

	if err := runPost(context.Background(), d, modalSubmit(t, "こんにちは")); err != nil {
		t.Fatal(err)
	}
	for _, r := range s.requests {
		if r.Method == http.MethodDelete {
			t.Errorf("deleted %s even though the history was empty", r.Path)
		}
	}
}
