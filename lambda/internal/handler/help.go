package handler

import (
	dgo "github.com/disgoorg/disgo/discord"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

const embedColor = 0x197A4B

// help は使い方を表示するコマンド。実行者にだけ見せる。
var help = Command{
	Mode: Sync,
	Definition: &dgo.SlashCommandCreate{
		Name:        "help",
		Description: "Shows how to use this bot",
		DescriptionLocalizations: map[dgo.Locale]string{
			dgo.LocaleJapanese: "このBotの使い方を表示します",
		},
		IntegrationTypes: []dgo.ApplicationIntegrationType{
			dgo.ApplicationIntegrationTypeGuildInstall,
			dgo.ApplicationIntegrationTypeUserInstall,
		},
		Contexts: []dgo.InteractionContextType{
			dgo.InteractionContextTypeGuild,
			dgo.InteractionContextTypeBotDM,
		},
	},
	Handle: func(dgo.Interaction) (*dgo.InteractionResponse, error) {
		resp := discord.Embed(helpEmbed(), true)
		return &resp, nil
	},
}

func helpEmbed() dgo.Embed {
	return dgo.Embed{
		Title:       "匿名Botの使い方",
		Description: "チャンネル内に匿名でメッセージを投稿できるBotです",
		Color:       embedColor,
		Fields: []dgo.EmbedField{
			{
				Name: "1. セットアップ",
				Value: "匿名メッセージを使いたいチャンネルで`/setup`を実行してください。\n" +
					"送信用のパネルが設置されます。\n" +
					"既定では管理者のみ実行できますが、サーバー設定で変更できます。",
			},
			{
				Name: "2. メッセージを送る",
				Value: "パネルの **メッセージを送信** ボタンを押し、フォームに内容を入力して送信します。\n" +
					"投稿されたメッセージに送信者の情報は残りません。",
			},
			{
				Name: "匿名の名前について",
				Value: "表示名とアイコンは毎日変わります。\n" +
					"組み合わせには限りがあるため、同じ名前でも別の人のことがあります。",
			},
			{
				Name: "コマンド一覧",
				Value: "`/help`: この使い方を表示します\n" +
					"`/ping`: 疎通確認をします\n" +
					"`/setup`: パネルを設置します",
			},
		},
	}
}
