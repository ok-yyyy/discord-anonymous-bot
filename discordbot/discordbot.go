package discordbot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
)

type Bot struct {
	client *bot.Client
	config Config
}

func New(cfg Config, r handler.Router) (*Bot, error) {
	client, err := disgo.New(cfg.BotToken,
		bot.WithLogger(slog.Default()),
		bot.WithEventListeners(r),
		bot.WithDefaultGateway(),
	)
	if err != nil {
		return nil, fmt.Errorf("create discord client: %w", err)
	}

	return &Bot{
		client: client,
		config: cfg,
	}, nil
}

func (b *Bot) Open(ctx context.Context) error {
	if err := b.client.OpenGateway(ctx); err != nil {
		return fmt.Errorf("open gateway: %w", err)
	}
	return nil
}

func (b *Bot) Close(ctx context.Context) {
	b.client.Close(ctx)
}

func (b *Bot) SyncCommands(ctx context.Context, commands []discord.ApplicationCommandCreate) error {
	if _, err := b.client.Rest.SetGlobalCommands(b.client.ApplicationID, commands, rest.WithCtx(ctx)); err != nil {
		return fmt.Errorf("set global commands: %w", err)
	}
	return nil
}
