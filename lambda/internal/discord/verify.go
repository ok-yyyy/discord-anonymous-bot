// Package discord はdisgoでそのまま扱えない部分を補う。
//
// 具体的にはLambda Function URLのリクエストをdisgoの署名検証に渡せる形へ変換する。
// Interactionの型やREST呼び出しはdisgoのものを直接使い、ここで包み直さない。
package discord

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/disgoorg/disgo/httpserver"
)

// ErrInvalidSignature は署名検証に失敗したことを表す。
// このエラーにはHTTP 401を返す。
var ErrInvalidSignature = errors.New("invalid request signature")

// verifier は署名検証の実装。disgoのものをそのまま使う。
var verifier httpserver.Verifier = httpserver.DefaultVerifier{}

// Function URLはヘッダ名を小文字にして渡す。
const (
	headerSignature = "x-signature-ed25519"
	headerTimestamp = "x-signature-timestamp"
)

// VerifiedBody はFunction URLのリクエストの署名を検証し、本文を返す。
//
// 本文は受け取ったバイト列のまま扱う。
// JSONとして読み直して組み立てると署名が合わなくなる。
func VerifiedBody(publicKey httpserver.PublicKey, req events.LambdaFunctionURLRequest) ([]byte, error) {
	body, err := decodeBody(req)
	if err != nil {
		return nil, err
	}
	if !verifySignature(publicKey, req, body) {
		return nil, ErrInvalidSignature
	}
	return body, nil
}

// verifySignature は署名が正しいかを返す。
//
// 署名長と非正規な署名はed25519.Verifyが弾くので、ここでは検査しない。
// 一方で公開鍵の長さが違うとed25519.Verifyはpanicするため、そこだけ確認する。
func verifySignature(publicKey httpserver.PublicKey, req events.LambdaFunctionURLRequest, body []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	timestamp := req.Headers[headerTimestamp]
	if timestamp == "" {
		return false
	}

	sig, err := hex.DecodeString(req.Headers[headerSignature])
	if err != nil {
		return false
	}

	// 署名の対象はタイムスタンプと本文を連結したもの。
	msg := make([]byte, 0, len(timestamp)+len(body))
	msg = append(msg, timestamp...)
	msg = append(msg, body...)

	return verifier.Verify(publicKey, msg, sig)
}

// decodeBody はリクエスト本文をバイト列として取り出す。
// Function URLは内容によって本文をbase64で渡してくる。
func decodeBody(req events.LambdaFunctionURLRequest) ([]byte, error) {
	if !req.IsBase64Encoded {
		return []byte(req.Body), nil
	}

	body, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return nil, fmt.Errorf("decode base64 body: %w", err)
	}
	return body, nil
}
