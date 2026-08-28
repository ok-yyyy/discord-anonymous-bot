package anonymous

import (
	"fmt"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

// SendMessage は、webhook経由で匿名メッセージを投稿する。
// 投稿者は salt と userID から導出した匿名の名前とアバターに差し替えられる。
func SendMessage(client *bot.Client, webhook *discord.IncomingWebhook, salt string, userID snowflake.ID, content string) error {
	username, avatarURL := anonymousIdentity(salt, userID.String(), time.Now())

	_, err := client.Rest.CreateWebhookMessage(webhook.ID(), webhook.Token,
		discord.NewWebhookMessageCreate().
			WithContent(content).
			WithUsername(username).
			WithAvatarURL(avatarURL).
			WithAllowedMentions(&discord.AllowedMentions{ // メンションはすべて無効
				Parse: []discord.AllowedMentionType{},
				Roles: []snowflake.ID{},
				Users: []snowflake.ID{},
			}),
		rest.CreateWebhookMessageParams{},
	)
	if err != nil {
		return fmt.Errorf("create webhook message: %w", err)
	}

	return nil
}
