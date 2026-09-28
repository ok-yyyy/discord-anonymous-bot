package handler

import (
	"context"
	"fmt"
	"log/slog"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

// reply は実行者に結果を伝える。
//
// 送り方はackの方式に合わせる（discord.AckShowsThinkingを参照）。
// 「考え中…」を出している場合はそれを本文で置き換える。放置すると消えないため、
// その場合は成否にかかわらず必ず呼ぶ必要がある。
//
// 無言ackの場合は新しくメッセージを送る。@originalはモーダルやボタンのある
// 元のメッセージを指すため、編集するとそちらが書き換わってしまう。
func reply(ctx context.Context, d *Deps, i dgo.Interaction, content string) error {
	if !discord.AckShowsThinking(i) {
		_, err := d.Rest.CreateFollowupMessage(d.ApplicationID, i.Token(), dgo.MessageCreate{
			Content:         content,
			Flags:           dgo.MessageFlagEphemeral,
			AllowedMentions: discord.NoMentions(),
		}, rest.WithCtx(ctx))
		if err != nil {
			return fmt.Errorf("create followup message: %w", err)
		}
		return nil
	}

	_, err := d.Rest.UpdateInteractionResponse(d.ApplicationID, i.Token(), dgo.MessageUpdate{
		Content:         &content,
		AllowedMentions: discord.NoMentions(),
	}, rest.WithCtx(ctx))
	if err != nil {
		return fmt.Errorf("update interaction response: %w", err)
	}
	return nil
}

// ReplyBestEffort は結果の通知を試み、失敗してもログに残すだけにする。
//
// 既に投稿やパネルの設置が済んだ後の通知に使う。ここでエラーを返すとその
// メッセージが失敗として扱われ、再実行で投稿が重複しかねない。
func ReplyBestEffort(ctx context.Context, d *Deps, i dgo.Interaction, content string) {
	if err := reply(ctx, d, i, content); err != nil {
		slog.ErrorContext(ctx, "failed to tell the user the result", "error", err)
	}
}
