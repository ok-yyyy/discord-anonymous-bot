package commands

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/anonymous"
	"github.com/ok-yyyy/discord-anonymous-bot/message"
)

func (h *handlers) handleOpenModal(_ discord.ButtonInteractionData, e *handler.ComponentEvent) error {
	// モーダルの表示
	return e.Modal(discord.NewModalCreate(anonymous.SubmitModalCustomID, "匿名メッセージの送信").
		AddLabel("メッセージ内容",
			discord.NewParagraphTextInput(anonymous.ContentInputCustomID).
				WithRequired(true).
				WithMaxLength(2000),
		),
	)
}

func (h *handlers) handleSubmitModal(e *handler.ModalEvent) error {
	channelID := e.Channel().ID()
	userID := e.User().ID
	content := e.Data.Text(anonymous.ContentInputCustomID)

	slog.Info("anonymous message received",
		slog.Any("guild_id", e.GuildID()),
		slog.Any("channel_id", channelID),
		slog.Any("user_id", userID),
		slog.String("user_name", e.User().EffectiveName()),
		slog.String("content", content),
	)

	// webhookを取得
	webhook, err := anonymous.EnsureWebhook(e.Client(), channelID)
	if err != nil {
		e.Client().Logger.Error("ensure webhook",
			slog.Any("err", err),
			slog.Any("channel_id", channelID),
		)
		return e.CreateMessage(
			message.ErrorMessageCreate("webhookの取得に失敗しました").WithEphemeral(true),
		)
	}

	// Webhookメッセージの送信
	if err := anonymous.SendMessage(e.Client(), webhook, h.cfg.AnonymousSalt, userID, content); err != nil {
		e.Client().Logger.Error("send anonymous message",
			slog.Any("err", err),
			slog.Any("channel_id", channelID),
		)
		return e.CreateMessage(
			message.ErrorMessageCreate("匿名メッセージの送信に失敗しました").WithEphemeral(true),
		)
	}

	// パネルのリセット
	if e.Message != nil {
		_ = e.Client().Rest.DeleteMessage(channelID, e.Message.ID)
		_, _ = e.Client().Rest.CreateMessage(channelID, anonymous.PanelMessageCreate())
	}

	return e.DeferUpdateMessage()
}
