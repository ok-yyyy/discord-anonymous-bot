package anonymous

import (
	"github.com/disgoorg/disgo/discord"
)

// PanelMessageCreate は、匿名メッセージの送信パネルを組み立てる。
func PanelMessageCreate() discord.MessageCreate {
	return discord.NewMessageCreate().
		WithComponents(
			discord.NewActionRow(
				discord.NewPrimaryButton("メッセージを送信", OpenModalCustomID),
			),
		)
}
