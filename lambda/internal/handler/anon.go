package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/anon"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

const (
	// customIDOpen はパネルのボタン、customIDSubmitはモーダルの識別子。
	customIDOpen   = "anon:open"
	customIDSubmit = "anon:submit"

	// fieldMessage はモーダルの入力欄の識別子。
	fieldMessage = "message"

	// maxMessageLength はDiscordのメッセージ本文の上限。
	maxMessageLength = 2000

	// panelLookback はパネルを掃除するときに遡って調べるメッセージ数。
	// 投稿ごとに張り替えるので、古いパネルは直近に収まる。
	panelLookback = 50
)

// openModal はパネルのボタンを押したときに入力欄を開く。
//
// モーダルはdeferredにできないため必ず同期で返す。
// ここで外部I/Oをすると3秒を超えてInteractionごと失敗する。
var openModal = Command{
	Mode: Sync,
	Handle: func(dgo.Interaction) (*dgo.InteractionResponse, error) {
		resp := discord.Modal(dgo.ModalCreate{
			CustomID: customIDSubmit,
			Title:    "匿名メッセージ",
			Components: []dgo.LayoutComponent{
				dgo.LabelComponent{
					Label: "本文",
					Component: dgo.TextInputComponent{
						CustomID:    fieldMessage,
						Style:       dgo.TextInputStyleParagraph,
						MaxLength:   maxMessageLength,
						Required:    true,
						Placeholder: "匿名で投稿する内容を入力してください",
					},
				},
			},
		})
		return &resp, nil
	},
}

// postAnonymousMessage はモーダルの内容を匿名で投稿する。
var postAnonymousMessage = Command{
	Mode: Async,
	// MODAL_SUBMITなので無言ackになる。投稿されたメッセージとパネルの
	// 張り替えで結果が分かるため、完了の通知も出さない。
	// 失敗したときだけfollowupで伝える。
	Validate: validateMessage,
	Work:     runPost,
}

// validateMessage はSQSに積む前の検査。不正な入力をworkerまで運ばない。
func validateMessage(i dgo.Interaction) error {
	content := messageOf(i)
	switch {
	case content == "":
		return userErrorf("本文が空です。")
	case utf8.RuneCountInString(content) > maxMessageLength:
		return userErrorf("本文が長すぎます。%d文字以内にしてください。", maxMessageLength)
	}
	return nil
}

func runPost(ctx context.Context, d *Deps, i dgo.Interaction) error {
	webhook, err := findWebhook(ctx, d, i.Channel().ID())
	if err != nil {
		return err
	}
	if webhook == nil {
		// 権限の無いチャンネルに投稿してしまわないよう、勝手に作り直さない。
		return userErrorf("このチャンネルの投稿設定が見つかりません。`/setup` をやり直してください。")
	}

	identity := anon.Derive(i.User().ID.String(), d.Now(), d.Salt)
	// 匿名メッセージとパネルの順序を保つためにWaitを付ける。
	_, err = d.Rest.CreateWebhookMessage(webhook.ID(), webhook.Token, dgo.WebhookMessageCreate{
		Content:   messageOf(i),
		Username:  identity.Name,
		AvatarURL: identity.AvatarURL,
		// 匿名投稿から@everyoneやロールメンションが飛ばないようにする。
		AllowedMentions: discord.NoMentions(),
	}, rest.CreateWebhookMessageParams{Wait: true}, rest.WithCtx(ctx))
	if err != nil {
		if discord.IsNotFound(err) {
			return userErrorf("投稿先のWebhookが削除されています。`/setup` をやり直してください。")
		}
		// 本文がエラーに混ざらないよう、err以外を足さない。
		return fmt.Errorf("execute webhook: %w", err)
	}

	// 投稿が済んだのでここから先は失敗してもこの処理は成功として扱う。
	// エラーを返すと再実行され、投稿が重複する。
	refreshPanel(ctx, d, i)
	return nil
}

// refreshPanel は投稿パネルを作り直し、チャンネルの一番下に置く。
//
// 新しいパネルを作ってから、それ以外のパネルを消す。
// 逆順にすると、削除に成功して作成に失敗した場合にパネルが1つも無い状態になり、
// /setup をやり直すまで投稿できなくなる。
func refreshPanel(ctx context.Context, d *Deps, i dgo.Interaction) {
	channelID := i.Channel().ID()

	created, err := d.Rest.CreateMessage(channelID, panelMessage(), rest.WithCtx(ctx))
	if err != nil {
		slog.ErrorContext(ctx, "failed to post a new panel", "error", err)
		return
	}

	deleteStalePanels(ctx, d, channelID, created.ID)
}

// deleteStalePanels は直近のメッセージから、keep以外のパネルを消す。
//
// モーダルを開いたメッセージだけを消すやり方だと、同じパネルから複数人が
// モーダルを開いたときにパネルが増える。後から送信した側の削除は404になる一方で、
// 新しいパネルはそれぞれ作られるため。直近をまとめて掃除して1枚に収束させる。
func deleteStalePanels(ctx context.Context, d *Deps, channelID, keep snowflake.ID) {
	messages, err := d.Rest.GetMessages(channelID, 0, 0, 0, panelLookback, rest.WithCtx(ctx))
	if err != nil {
		slog.ErrorContext(ctx, "failed to list messages while refreshing the panel", "error", err)
		return
	}

	for _, m := range messages {
		if m.ID == keep || !isOwnPanel(d, m) {
			continue
		}
		// 既に消えている場合は何もしなくてよい。
		if err := d.Rest.DeleteMessage(channelID, m.ID, rest.WithCtx(ctx)); err != nil && !discord.IsNotFound(err) {
			slog.ErrorContext(ctx, "failed to delete a stale panel", "error", err)
		}
	}
}

// isOwnPanel はBot自身が投稿したパネルかを返す。
//
// 匿名メッセージはWebhook経由で投稿しているため、WebhookIDで必ず除外する。
// これを忘れると利用者の投稿を消してしまう。
//
// Bot自身が投稿する非ephemeralなメッセージはパネルだけなので、それ以上は見ない。
// パネル以外を投稿するようになったら、ここに判定を足す必要がある。
func isOwnPanel(d *Deps, m dgo.Message) bool {
	return m.WebhookID == nil && m.Author.ID == d.ApplicationID
}

// messageOf はモーダルの入力値を取り出す。
func messageOf(i dgo.Interaction) string {
	modal, ok := i.(dgo.ModalSubmitInteraction)
	if !ok {
		return ""
	}
	return strings.TrimSpace(modal.Data.Text(fieldMessage))
}
