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

// Message はその場で本文を返す応答を組み立てる。
// ephemeralにすると実行者にだけ見える。
func Message(content string, ephemeral bool) dgo.InteractionResponse {
	return createMessage(dgo.MessageCreate{Content: content}, ephemeral)
}

// Embed はembedを1つ含む応答を組み立てる。
func Embed(embed dgo.Embed, ephemeral bool) dgo.InteractionResponse {
	return createMessage(dgo.MessageCreate{Embeds: []dgo.Embed{embed}}, ephemeral)
}

func createMessage(data dgo.MessageCreate, ephemeral bool) dgo.InteractionResponse {
	// こちらから送る文面でメンションが飛ばないようにする。
	// Parseを省くと "parse":null になり、Discordは未指定として扱う。
	// 空配列を明示しないとメンションが抑止されない。
	data.AllowedMentions = &dgo.AllowedMentions{Parse: []dgo.AllowedMentionType{}}
	if ephemeral {
		data.Flags = dgo.MessageFlagEphemeral
	}

	return dgo.InteractionResponse{Type: dgo.InteractionResponseTypeCreateMessage, Data: data}
}

// Respond はInteractionへの応答をFunction URLのレスポンスへ変換する。
func Respond(resp dgo.InteractionResponse) (events.LambdaFunctionURLResponse, error) {
	body, err := json.Marshal(resp)
	if err != nil {
		// ここで失敗すると応答を返せず、Discord側には失敗として見える。
		return Status(http.StatusInternalServerError), fmt.Errorf("encode interaction response: %w", err)
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
