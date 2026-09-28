// Command worker はSQSに積まれたInteractionを処理する。
//
// Discord APIを呼ぶ処理はすべてここで行う。
// interaction Lambdaは既に「考え中…」を返しているので、結果はfollowupで伝える。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	dgo "github.com/disgoorg/disgo/discord"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/config"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/handler"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/queue"
)

// tokenLifetime はInteractionのtokenが有効な時間。
// これを過ぎるとfollowupを送れないので、処理せずに捨てる。
const tokenLifetime = 15 * time.Minute

type app struct {
	deps *handler.Deps
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	deps := &handler.Deps{
		Rest:          discord.NewRest(cfg.BotToken),
		ApplicationID: cfg.ApplicationID,
		Salt:          cfg.AnonymousSalt,
		Now:           time.Now,
	}

	lambda.Start((&app{deps: deps}).handle)
}

func (a *app) handle(ctx context.Context, event events.SQSEvent) error {
	// バッチサイズは1にしているが、複数届いても順に処理できるようにしておく。
	for _, record := range event.Records {
		if err := a.process(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

// process は1件を処理する。
//
// 返すエラーは「このメッセージをDLQに送る」という意味になる。
// 再試行は二重投稿につながるため、キューのmaxReceiveCountは1にしてある。
func (a *app) process(ctx context.Context, record events.SQSMessage) error {
	var msg queue.Message
	if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
		return fmt.Errorf("decode queue message: %w", err)
	}

	// tokenが切れているとfollowupも送れない。処理しても誰にも届かないので捨てる。
	if age := time.Since(msg.ReceivedAt); age > tokenLifetime {
		slog.WarnContext(ctx, "discarded an interaction whose token had expired", "age", age.String())
		return nil
	}

	interaction, err := dgo.UnmarshalInteraction(msg.Interaction)
	if err != nil {
		// 本文には投稿内容が含まれうるのでログに出さない。
		return fmt.Errorf("decode interaction: %w", err)
	}

	cmd, ok := handler.Lookup(interaction)
	if !ok || cmd.Work == nil {
		// interaction Lambdaとworkerのデプロイがずれている状態。
		handler.ReplyBestEffort(ctx, a.deps, interaction, handler.MsgFailed)
		return fmt.Errorf("no async handler for interaction type %d", interaction.Type())
	}

	err = cmd.Work(ctx, a.deps, interaction)
	if err == nil {
		return nil
	}

	content, shown := handler.UserMessage(err)
	handler.ReplyBestEffort(ctx, a.deps, interaction, content)

	// 想定内の失敗 (権限不足、setup未実行など) は、伝えるだけで終える。
	// 再試行しても結果は変わらない。
	if shown {
		return nil
	}

	// 想定外の失敗。内容は伏せて伝え、メッセージはDLQに送る。
	slog.ErrorContext(ctx, "async handler failed", "interaction_type", interaction.Type(), "error", err)
	return err
}
