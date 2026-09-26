package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"

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
	channelID := i.Channel().ID()
	userID := i.User().ID
	content := messageOf(i)

	slog.Info("anonymous message received",
		slog.Any("guild_id", i.GuildID()),
		slog.Any("channel_id", channelID),
		slog.Any("user_id", userID),
		slog.String("user_name", i.User().EffectiveName()),
		slog.String("content", content),
	)

	webhook, err := findWebhook(ctx, d, channelID)
	if err != nil {
		return err
	}
	if webhook == nil {
		// 権限の無いチャンネルに投稿してしまわないよう、勝手に作り直さない。
		return userErrorf("このチャンネルの投稿設定が見つかりません。`/setup` をやり直してください。")
	}

	identity := anon.Derive(userID.String(), d.Now(), d.Salt)
	_, err = d.Rest.CreateWebhookMessage(webhook.ID(), webhook.Token, dgo.WebhookMessageCreate{
		Content:   content,
		Username:  identity.Name,
		AvatarURL: identity.AvatarURL,
		// 匿名投稿から@everyoneやロールメンションが飛ばないようにする。
		AllowedMentions: &dgo.AllowedMentions{Parse: []dgo.AllowedMentionType{}},
	}, rest.CreateWebhookMessageParams{}, rest.WithCtx(ctx))
	if err != nil {
		if isNotFound(err) {
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
func refreshPanel(ctx context.Context, d *Deps, i dgo.Interaction) {
	modal, ok := i.(dgo.ModalSubmitInteraction)
	if !ok || modal.Message == nil {
		// ボタン以外から開かれたモーダル。消すべきパネルが分からない。
		return
	}

	channelID := i.Channel().ID()
	if _, err := d.Rest.CreateMessage(channelID, panelMessage(), rest.WithCtx(ctx)); err != nil {
		slog.ErrorContext(ctx, "failed to post a new panel", "error", err)
		return
	}

	// 既に消えている場合 (古いパネルから開かれた等) は何もしなくてよい。
	err := d.Rest.DeleteMessage(channelID, modal.Message.ID, rest.WithCtx(ctx))
	if err != nil && !isNotFound(err) {
		slog.ErrorContext(ctx, "failed to delete the old panel", "error", err)
	}
}

// messageOf はモーダルの入力値を取り出す。
func messageOf(i dgo.Interaction) string {
	modal, ok := i.(dgo.ModalSubmitInteraction)
	if !ok {
		return ""
	}
	return strings.TrimSpace(modal.Data.Text(fieldMessage))
}
