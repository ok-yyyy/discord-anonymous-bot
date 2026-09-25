// Command interaction はDiscordからのInteractionをLambda Function URLで受ける。
//
// ここでの仕事は署名の検証と振り分けだけ。
// Discordは3秒以内の応答を求めるため、時間のかかる処理はSQSに積んでworkerに任せる。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	dgo "github.com/disgoorg/disgo/discord"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/config"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

// msgNotImplemented はまだ実装していないInteractionへの応答。
//
// コマンドを未登録の間は届かないはずだが、無応答にすると実行者には
// 「アプリケーションが応答しませんでした」とだけ出て原因が分からない。
const msgNotImplemented = "このコマンドは使えません。"

type app struct {
	cfg *config.Interaction
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.LoadInteraction()
	if err != nil {
		// 設定が欠けたまま動くと署名検証をすり抜けかねないので起動させない。
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	lambda.Start((&app{cfg: cfg}).handle)
}

func (a *app) handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	body, err := discord.VerifiedBody(a.cfg.PublicKey, req)
	if err != nil {
		if errors.Is(err, discord.ErrInvalidSignature) {
			// 401以外を返すとDiscordのエンドポイント検証に通らない。
			return discord.Status(http.StatusUnauthorized), nil
		}
		slog.WarnContext(ctx, "failed to read the request body", "error", err)
		return discord.Status(http.StatusBadRequest), nil
	}

	interaction, err := dgo.UnmarshalInteraction(body)
	if err != nil {
		slog.WarnContext(ctx, "failed to parse the interaction", "error", err)
		return discord.Status(http.StatusBadRequest), nil
	}

	return discord.Respond(a.route(ctx, interaction))
}

// route はInteractionに対する応答を決める。
func (a *app) route(ctx context.Context, i dgo.Interaction) dgo.InteractionResponse {
	// エンドポイント検証のPING。ルーティング表は通さない。
	if i.Type() == dgo.InteractionTypePing {
		return discord.Pong()
	}

	// TODO: Registryを引いて Sync / Async に振り分ける。
	slog.WarnContext(ctx, "no handler for the interaction", "interaction_type", i.Type())
	return discord.Message(msgNotImplemented, true)
}
