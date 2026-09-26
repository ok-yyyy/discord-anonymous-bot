package handler

import (
	"context"
	"fmt"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/omit"
)

// setup はチャンネルに投稿パネルを設置するコマンド。
//
// Webhookの確認・作成とパネルの投稿でDiscord APIを複数回呼ぶためAsync。
var setup = Command{
	Mode: Async,
	Definition: &dgo.SlashCommandCreate{
		Name:        "setup",
		Description: "Setup anonymous channel",
		DescriptionLocalizations: map[dgo.Locale]string{
			dgo.LocaleJapanese: "匿名チャンネルの設定を行います",
		},
		// 一般ユーザーが実行できないようにする。サーバー設定で変更できる。
		DefaultMemberPermissions: omit.NewPtr(dgo.PermissionManageGuild),
		// Webhookを作るのでサーバー内限定。DMでは機能しない。
		IntegrationTypes: []dgo.ApplicationIntegrationType{
			dgo.ApplicationIntegrationTypeGuildInstall,
		},
		Contexts: []dgo.InteractionContextType{
			dgo.InteractionContextTypeGuild,
		},
	},
	Work: runSetup,
}

func runSetup(ctx context.Context, d *Deps, i dgo.Interaction) error {
	channelID := i.Channel().ID()

	// パネルを置く前にWebhookを用意する。
	if _, err := ensureWebhook(ctx, d, channelID); err != nil {
		return err
	}

	_, err := d.Rest.CreateMessage(channelID, panelMessage(), rest.WithCtx(ctx))
	if err != nil {
		if isForbidden(err) {
			return userErrorf("パネルを投稿できませんでした。Botにこのチャンネルへの投稿権限があるか確認してください。")
		}
		return fmt.Errorf("post panel: %w", err)
	}

	// パネルは設置済みなので、通知に失敗してもこの処理は成功として扱う。
	replyBestEffort(ctx, d, i, "投稿パネルを設置しました。")
	return nil
}

// panelMessage は設置するパネル。
func panelMessage() dgo.MessageCreate {
	return dgo.MessageCreate{
		Components: []dgo.LayoutComponent{
			dgo.NewActionRow(dgo.NewPrimaryButton("メッセージを送信", customIDOpen)),
		},
	}
}
