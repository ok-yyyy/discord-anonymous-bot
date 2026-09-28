package discord

import (
	"github.com/disgoorg/disgo/rest"
)

// NewRest はDiscord RESTクライアントを作る。
//
// **メンションを飛ばさないことを既定にする。** disgoの既定は逆で、
// AllowedMentionsを指定しなかった送信に users / roles / everyone すべてを
// 許可する値を埋める。指定漏れが1か所あるだけで@everyoneが飛ぶため、
// 呼び出し側の注意に頼らず既定を反転させておく。
//
// 個別の送信でAllowedMentionsを明示すればそちらが優先される。
func NewRest(botToken string, opts ...rest.ClientConfigOpt) rest.Rest {
	return rest.New(
		rest.NewClient(botToken, opts...),
		rest.WithDefaultAllowedMentions(*NoMentions()),
	)
}
