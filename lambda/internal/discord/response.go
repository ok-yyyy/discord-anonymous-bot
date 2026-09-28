package discord

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	dgo "github.com/disgoorg/disgo/discord"
)

// Pong はPINGへの応答を返す。Discordのエンドポイント検証はこれで通る。
func Pong() dgo.InteractionResponse {
	return dgo.InteractionResponse{Type: dgo.InteractionResponseTypePong}
}

// NoMentions はメンションを一切許可しない設定を返す。
//
// Parseを省くと "parse":null になり、Discordは未指定として扱う。
// 空配列を明示しないとメンションが抑止されない。
func NoMentions() *dgo.AllowedMentions {
	return &dgo.AllowedMentions{Parse: []dgo.AllowedMentionType{}}
}

// Message はその場で本文を返す応答を組み立てる。
func Message(content string) dgo.InteractionResponse {
	return createMessage(dgo.MessageCreate{Content: content})
}

// Embed はembedを1つ含む応答を組み立てる。
func Embed(embed dgo.Embed) dgo.InteractionResponse {
	return createMessage(dgo.MessageCreate{Embeds: []dgo.Embed{embed}})
}

// createMessage はその場で返す応答を組み立てる。
//
// 投稿内容が公開チャンネルに漏れないよう、実行者にだけ見せる。
// 公開したい応答が出てきたら、そのときにフラグを選べるようにする。
func createMessage(data dgo.MessageCreate) dgo.InteractionResponse {
	data.AllowedMentions = NoMentions()
	data.Flags = dgo.MessageFlagEphemeral

	return dgo.InteractionResponse{Type: dgo.InteractionResponseTypeCreateMessage, Data: data}
}

// Respond はInteractionへの応答をFunction URLのレスポンスへ変換する。
func Respond(resp dgo.InteractionResponse) (events.LambdaFunctionURLResponse, error) {
	body, err := json.Marshal(resp)
	if err != nil {
		// errを返すとLambdaは応答値を捨てるので、ゼロ値でよい。
		// Discord側には失敗として見える。
		return events.LambdaFunctionURLResponse{}, fmt.Errorf("encode interaction response: %w", err)
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

// Status は本文のないレスポンスを返す。
func Status(code int) events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{StatusCode: code}
}

// Modal は入力ダイアログを開く応答を組み立てる。
//
// deferredにできないので、これを返すハンドラは同期で完結させる必要がある。
func Modal(modal dgo.ModalCreate) dgo.InteractionResponse {
	return dgo.InteractionResponse{Type: dgo.InteractionResponseTypeModal, Data: modal}
}

// Ack はInteractionを受け付けたことを伝える応答を返す。
//
// どの方式になるかはInteractionの種類で決まる。type 6（無言ack）は
// コンポーネントとMODAL_SUBMITでしか使えず、スラッシュコマンドでは
// type 5（deferred）しか選べない。
func Ack(i dgo.Interaction) dgo.InteractionResponse {
	if !AckShowsThinking(i) {
		return dgo.InteractionResponse{Type: dgo.InteractionResponseTypeDeferredUpdateMessage}
	}

	// 投稿内容が公開チャンネルに漏れないよう、実行者にだけ見せる。
	return dgo.InteractionResponse{
		Type: dgo.InteractionResponseTypeDeferredCreateMessage,
		Data: dgo.MessageCreate{Flags: dgo.MessageFlagEphemeral},
	}
}

// AckShowsThinking はackが「考え中…」を表示するかどうかを返す。
//
// trueのときは@originalがその「考え中…」を指すので、結果はそれを編集して伝える。
// 編集しないと「考え中…」が残り続ける。
//
// falseのときは@originalがモーダルやボタンのある元のメッセージを指す。
// 編集するとそのメッセージが書き換わってしまうので、結果はfollowupで伝える。
func AckShowsThinking(i dgo.Interaction) bool {
	switch i.Type() {
	case dgo.InteractionTypeComponent, dgo.InteractionTypeModalSubmit:
		return false
	default:
		return true
	}
}
