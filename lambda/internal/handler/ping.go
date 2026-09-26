package handler

import (
	dgo "github.com/disgoorg/disgo/discord"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

// ping は疎通確認用のコマンド。外部I/Oが無いので同期で返す。
var ping = Command{
	Mode: Sync,
	Definition: &dgo.SlashCommandCreate{
		Name:        "ping",
		Description: "Responds with pong",
		DescriptionLocalizations: map[dgo.Locale]string{
			dgo.LocaleJapanese: "pongと返します",
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
		resp := discord.Message("pong", true)
		return &resp, nil
	},
}
