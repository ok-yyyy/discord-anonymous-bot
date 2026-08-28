package commands

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/omit"
	"github.com/ok-yyyy/discord-anonymous-bot/anonymous"
	"github.com/ok-yyyy/discord-anonymous-bot/message"
)

var setupCommand = discord.SlashCommandCreate{
	Name:        "setup",
	Description: "Setup anonymous channel",
	DescriptionLocalizations: map[discord.Locale]string{
		discord.LocaleJapanese: "匿名チャンネルの設定を行います",
	},
	DefaultMemberPermissions: omit.NewPtr(discord.PermissionAdministrator),
	IntegrationTypes: []discord.ApplicationIntegrationType{
		discord.ApplicationIntegrationTypeGuildInstall,
	},
	Contexts: []discord.InteractionContextType{
		discord.InteractionContextTypeGuild,
	},
}

func (h *handlers) handleSetup(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	channelID := e.Channel().ID()

	// botの権限チェック
	const requiredPerms = discord.PermissionViewChannel |
		discord.PermissionSendMessages |
		discord.PermissionReadMessageHistory |
		discord.PermissionManageWebhooks
	perms := e.AppPermissions()
	if perms != nil && perms.Missing(requiredPerms) {
		missing := requiredPerms.Remove(*perms)
		return e.CreateMessage(
			message.ErrorMessageCreatef("botに必要な権限がありません: %s", missing).WithEphemeral(true),
		)
	}

	// webhookの準備
	_, err := anonymous.EnsureWebhook(e.Client(), channelID)
	if err != nil {
		e.Client().Logger.Error("ensure webhook",
			slog.Any("err", err),
			slog.Any("channel_id", channelID),
		)
		return e.CreateMessage(
			message.ErrorMessageCreate("webhookの作成に失敗しました").WithEphemeral(true),
		)
	}

	// パネルの設置
	_, err = e.Client().Rest.CreateMessage(channelID, anonymous.PanelMessageCreate())
	if err != nil {
		e.Client().Logger.Error("create panel",
			slog.Any("err", err),
			slog.Any("channel_id", channelID),
		)
		return e.CreateMessage(
			message.ErrorMessageCreate("パネルの作成に失敗しました").WithEphemeral(true),
		)
	}

	return e.CreateMessage(
		message.SuccessMessageCreate("設定が完了しました").WithEphemeral(true),
	)
}
