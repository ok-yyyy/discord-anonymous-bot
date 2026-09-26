// Package handler はコマンドのルーティング表と、各コマンドの処理をまとめる。
//
// interaction LambdaとworkerLambdaは同じ表を参照する。
// 前者はModeを見て「その場で返すかSQSに積むか」を決め、後者は積まれたものを処理する。
// 新しいコマンドを足すときはRegistryに1行足す。
package handler

import (
	"maps"
	"slices"

	dgo "github.com/disgoorg/disgo/discord"
)

// Mode はコマンドをどちらのLambdaで処理するかを表す。
type Mode int

const (
	// Sync はinteraction Lambda内で完結させる。
	Sync Mode = iota
	// Async はSQSに積み、worker Lambdaで処理する。
	Async
)

// Command は1つのInteractionに対する処理の定義。
type Command struct {
	Mode Mode

	// Definition はDiscordに登録するスラッシュコマンドの定義。
	// ボタンやモーダルのように登録が要らないものはnilにする。
	Definition *dgo.SlashCommandCreate

	// Handle はMode == Syncのときの処理。
	//
	// context.Contextを渡していないのは意図的で、ネットワークI/Oを書けないようにするため。
	// 3秒以内に返せなくなった時点でAsyncにする。
	Handle func(dgo.Interaction) (*dgo.InteractionResponse, error)
}

// Registry はInteractionの識別子から処理を引く表。
//
// キーはスラッシュコマンド名、またはコンポーネント/モーダルのcustom_id。
var Registry = map[string]Command{
	"ping": ping,
	"help": help,
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
