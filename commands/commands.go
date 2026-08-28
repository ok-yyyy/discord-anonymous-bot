package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/anonymous"
	"github.com/ok-yyyy/discord-anonymous-bot/discordbot"
)

var Commands = []discord.ApplicationCommandCreate{
	helpCommand,
	pingCommand,
	setupCommand,
}

// handlers は、設定を参照する各ハンドラの受け皿。
type handlers struct {
	cfg discordbot.Config
}

func RegisterRoutes(r *handler.Mux, cfg discordbot.Config) {
	h := &handlers{
		cfg: cfg,
	}

	r.SlashCommand("/help", h.handleHelp)
	r.SlashCommand("/ping", h.handlePing)
	r.SlashCommand("/setup", h.handleSetup)
	r.ButtonComponent(anonymous.OpenModalCustomID, h.handleOpenModal)
	r.Modal(anonymous.SubmitModalCustomID, h.handleSubmitModal)
}
