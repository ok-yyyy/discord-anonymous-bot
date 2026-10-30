// Package config は環境変数の読み取りと検証をまとめる。
//
// 各mainは起動時にここを一度だけ呼び、必須値が欠けていればその場で終了する。
//
// タグにはrequiredではなくnotEmptyを使う。
// requiredは「変数が設定されていること」しか見ないため、空文字が入っていても通ってしまう。
package config

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/caarlos0/env/v11"
)

// Interaction はinteraction Lambdaが必要とする設定。
//
// Botトークンとsaltは渡さない。このLambdaは署名検証とenqueueしかしないので、
// 漏れる面を狭くしておく。
type Interaction struct {
	PublicKey ed25519.PublicKey `env:"DISCORD_PUBLIC_KEY,notEmpty"` // PublicKey はInteractionの署名を検証する公開鍵
	QueueURL  string            `env:"QUEUE_URL,notEmpty"`          // QueueURL は非同期処理を積むSQSキュー
}

// LoadInteraction はinteraction Lambdaの設定を環境変数から読む。
func LoadInteraction() (*Interaction, error) {
	return load[Interaction]()
}

// load は環境変数を読み込んで検証する。
func load[T any]() (*T, error) {
	var cfg T
	err := env.ParseWithOptions(&cfg, env.Options{FuncMap: map[reflect.Type]env.ParserFunc{
		reflect.TypeFor[ed25519.PublicKey](): parsePublicKey,
	}})
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return &cfg, nil
}

// parsePublicKey は16進文字列からEd25519の公開鍵を読む。
//
// envはパーサの戻り値をそのままフィールドに入れるので、[]byteではなくed25519.PublicKeyとして返す。
//
// エラーに値の中身を含めない。
// 公開鍵の欄に誤って秘匿値を設定した場合にログへ流さないため。
func parsePublicKey(v string) (any, error) {
	key, err := hex.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("not a hex string: %w", err)
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("must be %d bytes, got %d", ed25519.PublicKeySize, len(key))
	}

	return ed25519.PublicKey(key), nil
}
