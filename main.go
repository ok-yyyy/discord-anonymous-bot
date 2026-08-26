package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/commands"
	"github.com/ok-yyyy/discord-anonymous-bot/discordbot"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("starting application")

	if err := run(ctx); err != nil {
		slog.Error("application failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("application finished successfully")
}

func run(ctx context.Context) error {
	cfg, err := discordbot.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cr := handler.New()
	commands.RegisterRoutes(cr, cfg)

	bot, err := discordbot.New(cfg, cr)
	if err != nil {
		return fmt.Errorf("new discord bot: %w", err)
	}

	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		bot.Close(closeCtx)
	}()

	if err := bot.SyncCommands(ctx, commands.Commands); err != nil {
		return err
	}

	openCtx, openCancel := context.WithTimeout(ctx, 10*time.Second)
	defer openCancel()
	if err := bot.Open(openCtx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
