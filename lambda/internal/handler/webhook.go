package handler

import (
	"context"
	"fmt"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/discord"
)

// webhookName は作成するWebhookの名前。
// 固定にしておくことで、何度/setupを実行しても使い回せる。
const webhookName = "anonymous-bot"

// findWebhook はこのアプリが作ったWebhookをチャンネルから探す。
// 見つからない場合はnilを返す (エラーにはしない) 。
func findWebhook(ctx context.Context, d *Deps, channelID snowflake.ID) (*dgo.IncomingWebhook, error) {
	webhooks, err := d.Rest.GetWebhooks(channelID, rest.WithCtx(ctx))
	if err != nil {
		if discord.IsForbidden(err) {
			return nil, userErrorf("このチャンネルのWebhookを確認できませんでした。Botに「ウェブフックの管理」権限があるか確認してください。")
		}
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	for _, w := range webhooks {
		incoming, ok := w.(dgo.IncomingWebhook)
		if !ok {
			continue
		}
		// 他アプリや手動で作られたWebhookを勝手に使わない。
		// tokenが空のものは実行できない (別サーバーのフォロー用など) 。
		if incoming.ApplicationID != nil && *incoming.ApplicationID == d.ApplicationID && incoming.Token != "" {
			return &incoming, nil
		}
	}
	return nil, nil
}

// ensureWebhook はWebhookが無ければ作る。
func ensureWebhook(ctx context.Context, d *Deps, channelID snowflake.ID) (*dgo.IncomingWebhook, error) {
	webhook, err := findWebhook(ctx, d, channelID)
	if err != nil {
		return nil, err
	}
	if webhook != nil {
		return webhook, nil
	}

	created, err := d.Rest.CreateWebhook(channelID, dgo.WebhookCreate{Name: webhookName}, rest.WithCtx(ctx))
	if err != nil {
		if discord.IsForbidden(err) {
			return nil, userErrorf("Webhookを作成できませんでした。Botに「ウェブフックの管理」権限があるか確認してください。")
		}
		return nil, fmt.Errorf("create webhook: %w", err)
	}
	return created, nil
}
