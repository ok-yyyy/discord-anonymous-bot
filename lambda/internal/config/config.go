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

	"github.com/caarlos0/env/v11"
)

// Interaction はinteraction Lambdaが必要とする設定。
//
// Botトークンとsaltは渡さない。このLambdaは署名検証とenqueueしかしないので、
// 漏れる面を狭くしておく。
type Interaction struct {
	PublicKey PublicKey `env:"DISCORD_PUBLIC_KEY,notEmpty"` // PublicKey はInteractionの署名を検証する公開鍵
	QueueURL  string    `env:"QUEUE_URL,notEmpty"`          // QueueURL は非同期処理を積むSQSキュー
}

// LoadInteraction はinteraction Lambdaの設定を環境変数から読む。
func LoadInteraction() (*Interaction, error) {
	return load[Interaction]()
}

// load は環境変数を読み込んで検証する。
func load[T any]() (*T, error) {
	var cfg T
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return &cfg, nil
}

// PublicKey はEd25519の公開鍵。16進文字列として環境変数から読む。
//
// ed25519.PublicKeyをそのままフィールドに使うとenvが[]byteの各要素を
// 数値として読もうとして失敗するため、独自の型にしてUnmarshalTextを持たせる。
type PublicKey ed25519.PublicKey

// UnmarshalText はenvがフィールドを埋めるときに呼ばれる。
//
// エラーに値の中身を含めない。
// 公開鍵の欄に誤って秘匿値を設定した場合にログへ流さないため。
func (k *PublicKey) UnmarshalText(text []byte) error {
	key, err := hex.DecodeString(string(text))
	if err != nil {
		return fmt.Errorf("not a hex string: %w", err)
	}
	if len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("must be %d bytes, got %d", ed25519.PublicKeySize, len(key))
	}

	*k = PublicKey(key)
	return nil
}
