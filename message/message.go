package message

import (
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
)

const (
	ColorSuccess = 0x197A4B
	ColorError   = 0xCE0000
)

func SuccessMessageCreate(msg string) discord.MessageCreate {
	return discord.NewMessageCreate().
		WithEmbeds(
			discord.NewEmbed().
				WithDescription(msg).
				WithColor(ColorSuccess).
				WithTimestamp(time.Now()),
		)
}

func SuccessMessageCreatef(msg string, args ...any) discord.MessageCreate {
	return SuccessMessageCreate(fmt.Sprintf(msg, args...))
}

func ErrorMessageCreate(msg string) discord.MessageCreate {
	return discord.NewMessageCreate().
		WithEmbeds(
			discord.NewEmbed().
				WithDescription(msg).
				WithColor(ColorError).
				WithTimestamp(time.Now()),
		)
}

func ErrorMessageCreatef(msg string, args ...any) discord.MessageCreate {
	return ErrorMessageCreate(fmt.Sprintf(msg, args...))
}
