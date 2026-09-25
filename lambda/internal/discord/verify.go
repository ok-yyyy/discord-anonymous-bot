// Package discord はdisgoでそのまま扱えない部分を補う。
//
// 具体的にはLambda Function URLのリクエストをdisgoの署名検証に渡せる形へ変換する。
// Interactionの型やREST呼び出しはdisgoのものを直接使い、ここで包み直さない。
package discord

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/disgoorg/disgo/httpserver"
)

// ErrInvalidSignature は署名検証に失敗したことを表す。
// このエラーにはHTTP 401を返す。
var ErrInvalidSignature = errors.New("invalid request signature")

// verifier は署名検証の実装。disgoのものをそのまま使う。
var verifier httpserver.Verifier = httpserver.DefaultVerifier{}

// VerifiedBody はFunction URLのリクエストの署名を検証し、本文を返す。
//
// 本文は受け取ったバイト列のまま扱う。
// JSONとして読み直して組み立てると署名が合わなくなる。
func VerifiedBody(publicKey httpserver.PublicKey, req events.LambdaFunctionURLRequest) ([]byte, error) {
	body, err := decodeBody(req)
	if err != nil {
		return nil, err
	}

	// disgoのVerifyRequestは署名長と非正規な署名の検査も行うため、
	// 自前で検証せずhttp.Requestを組み立てて渡す。
	r, err := asHTTPRequest(req, body)
	if err != nil {
		return nil, err
	}
	if !httpserver.VerifyRequest(verifier, r, publicKey) {
		return nil, ErrInvalidSignature
	}

	return body, nil
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

// asHTTPRequest は署名検証に渡すためだけのhttp.Requestを組み立てる。
// 検証に使うのは2つのヘッダと本文だけなので、URLやメソッドは形式的なもの。
func asHTTPRequest(req events.LambdaFunctionURLRequest, body []byte) (*http.Request, error) {
	r, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request for verification: %w", err)
	}

	// Function URLはヘッダ名を小文字にして渡す。
	r.Header.Set("X-Signature-Ed25519", req.Headers["x-signature-ed25519"])
	r.Header.Set("X-Signature-Timestamp", req.Headers["x-signature-timestamp"])
	return r, nil
}
