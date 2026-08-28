package anonymous

import (
	"fmt"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// FindOwnWebhook は、指定したチャンネルから自身が作成したwebhookを探して返す。
// 見つからない場合は (nil, nil) を返す。
func FindOwnWebhook(client *bot.Client, channelID snowflake.ID) (*discord.IncomingWebhook, error) {
	webhooks, err := client.Rest.GetWebhooks(channelID)
	if err != nil {
		return nil, fmt.Errorf("get webhooks: %w", err)
	}

	for _, wh := range webhooks {
		incoming, ok := wh.(discord.IncomingWebhook)
		if !ok {
			continue
		}
		if incoming.User.ID == client.ApplicationID {
			return &incoming, nil
		}
	}

	return nil, nil
}

// EnsureWebhook は、指定したチャンネルに自身が作成したwebhookがあればそれを返し、なければ新しく作成して返す。
func EnsureWebhook(client *bot.Client, channelID snowflake.ID) (*discord.IncomingWebhook, error) {
	webhook, err := FindOwnWebhook(client, channelID)
	if err != nil {
		return nil, fmt.Errorf("find own webhook: %w", err)
	}
	if webhook != nil {
		return webhook, nil
	}

	webhook, err = client.Rest.CreateWebhook(channelID, discord.WebhookCreate{
		Name: WebhookName,
	})
	if err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return webhook, nil
}
