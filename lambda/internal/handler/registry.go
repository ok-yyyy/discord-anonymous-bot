// Package handler はコマンドのルーティング表と、各コマンドの処理をまとめる。
//
// interaction LambdaとworkerLambdaは同じ表を参照する。
// 前者はRespondで応答を決め、Workがあればその前にSQSへ積む。後者は積まれたものを処理する。
// 新しいコマンドを足すときはRegistryに1行足す。
package handler

import (
	"context"
	"maps"
	"slices"
	"time"

	dgo "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

// Deps はWorkが使う依存。
type Deps struct {
	Rest          rest.Rest
	ApplicationID snowflake.ID
	// Salt は匿名IDの導出に使う。ログに出さないこと。
	Salt string
	// Now は現在時刻。テストで固定できるようにしている。
	Now func() time.Time
}

// Command は1つのInteractionに対する処理の定義。
type Command struct {
	// Definition はDiscordに登録するスラッシュコマンドの定義。
	// ボタンやモーダルのように登録が要らないものはnilにする。
	Definition *dgo.SlashCommandCreate

	// Respond はinteraction Lambdaで実行し、Discordへの応答を返す。
	//
	// 入力を拒否するときは必ずerrorを返す。応答として返すとWorkが実行されてしまう。
	// UserErrorならその内容が実行者に表示される。
	//
	// Workを持つコマンドはdiscord.Ackを返す。他の応答にするとworkerの返信先がずれるうえ、
	// Discordに拒否される応答だと、実行者には失敗と見えるのにWorkだけが動く。
	//
	// context.Contextを渡していないのは意図的で、ネットワークI/Oを書けないようにするため。
	// 3秒以内に返せなくなった時点でWorkに移す。
	Respond func(dgo.Interaction) (dgo.InteractionResponse, error)

	// Work はworker Lambdaで実行する処理。nilでなければRespondの後でSQSに積む。
	Work func(context.Context, *Deps, dgo.Interaction) error
}

// Registry はInteractionの識別子から処理を引く表。
//
// キーはスラッシュコマンド名、またはコンポーネント/モーダルのcustom_id。
var Registry = map[string]Command{
	"ping":         ping,
	"help":         help,
	"setup":        setup,
	customIDOpen:   openModal,
	customIDSubmit: postAnonymousMessage,
}

// Lookup はInteractionに対応する処理を返す。
func Lookup(i dgo.Interaction) (Command, bool) {
	cmd, ok := Registry[identifier(i)]
	return cmd, ok
}

// identifier はルーティングに使うキーを取り出す。
func identifier(i dgo.Interaction) string {
	switch i := i.(type) {
	case dgo.ApplicationCommandInteraction:
		return i.Data.CommandName()
	case dgo.ComponentInteraction:
		return i.Data.CustomID()
	case dgo.ModalSubmitInteraction:
		return i.Data.CustomID
	default:
		return ""
	}
}

// Definitions はDiscordに登録するコマンド定義を返す。
//
// mapの反復順は不定なので、登録内容が毎回同じになるよう名前順に並べる。
func Definitions() []dgo.ApplicationCommandCreate {
	defs := make([]dgo.ApplicationCommandCreate, 0, len(Registry))
	for _, name := range slices.Sorted(maps.Keys(Registry)) {
		if def := Registry[name].Definition; def != nil {
			defs = append(defs, *def)
		}
	}
	return defs
}
