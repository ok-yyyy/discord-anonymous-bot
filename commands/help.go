package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/message"
)

const repositoryURL = "https://github.com/ok-yyyy/discord-anonymous-bot"

var helpCommand = discord.SlashCommandCreate{
	Name:        "help",
	Description: "Shows how to use this bot",
	DescriptionLocalizations: map[discord.Locale]string{
		discord.LocaleJapanese: "このBotの使い方を表示します",
	},
	IntegrationTypes: []discord.ApplicationIntegrationType{
		discord.ApplicationIntegrationTypeGuildInstall,
		discord.ApplicationIntegrationTypeUserInstall,
	},
	Contexts: []discord.InteractionContextType{
		discord.InteractionContextTypeGuild,
		discord.InteractionContextTypeBotDM,
	},
}

func (h *handlers) handleHelp(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	embed := discord.NewEmbed().
		WithTitle("匿名Botの使い方").
		WithDescription("チャンネル内に匿名でメッセージを投稿できるBotです。").
		WithColor(message.ColorSuccess).
		AddField(
			"1. セットアップ",
			"匿名メッセージを使いたいチャンネルで`/setup`を実行してください。\n送信用のパネルが設置されます。\n既定では管理者のみ実行できますが、サーバー設定で変更できます。",
			false,
		).
		AddField(
			"2. メッセージを送る",
			"パネルの **メッセージを送信** ボタンを押し、フォームに内容を入力して送信します。\n投稿されたメッセージに送信者の情報は残りません。",
			false,
		).
		AddField(
			"コマンド一覧",
			"`/help`: この使い方を表示します\n`/setup`: パネルを設置します\n`/ping`: 疎通確認をします",
			false,
		).
		AddField(
			"リポジトリ",
			"[GitHub]("+repositoryURL+")",
			false,
		)

	return e.CreateMessage(
		discord.NewMessageCreate().
			WithEmbeds(embed).
			WithEphemeral(true),
	)
}
