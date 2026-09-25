package discord

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

const timestamp = "1758153600"

// signed は正しく署名されたリクエストを組み立てる。
func signed(t *testing.T, priv ed25519.PrivateKey, body string, encode bool) events.LambdaFunctionURLRequest {
	t.Helper()

	sig := ed25519.Sign(priv, append([]byte(timestamp), body...))
	req := events.LambdaFunctionURLRequest{
		// Function URLはヘッダ名を小文字にして渡す。
		Headers: map[string]string{
			"x-signature-ed25519":   hex.EncodeToString(sig),
			"x-signature-timestamp": timestamp,
		},
		Body: body,
	}
	if encode {
		req.Body = base64.StdEncoding.EncodeToString([]byte(body))
		req.IsBase64Encoded = true
	}
	return req
}

func keyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestVerifiedBody(t *testing.T) {
	pub, priv := keyPair(t)
	const body = `{"type":1}`

	got, err := VerifiedBody(pub, signed(t, priv, body, false))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("body = %q, want %q", got, body)
	}
}

// Function URLは本文をbase64で渡すことがある。署名は復号後のバイト列に対して行う。
func TestVerifiedBodyDecodesBase64(t *testing.T) {
	pub, priv := keyPair(t)
	const body = `{"type":1}`

	got, err := VerifiedBody(pub, signed(t, priv, body, true))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("body = %q, want %q", got, body)
	}
}

func TestVerifiedBodyRejectsBadSignature(t *testing.T) {
	pub, priv := keyPair(t)
	otherPub, _ := keyPair(t)

	tests := []struct {
		name   string
		key    ed25519.PublicKey
		mutate func(*events.LambdaFunctionURLRequest)
	}{
		{"tampered body", pub, func(r *events.LambdaFunctionURLRequest) {
			r.Body = `{"type":2}`
		}},
		{"tampered timestamp", pub, func(r *events.LambdaFunctionURLRequest) {
			r.Headers["x-signature-timestamp"] = "1758153601"
		}},
		{"missing signature", pub, func(r *events.LambdaFunctionURLRequest) {
			delete(r.Headers, "x-signature-ed25519")
		}},
		{"missing timestamp", pub, func(r *events.LambdaFunctionURLRequest) {
			delete(r.Headers, "x-signature-timestamp")
		}},
		{"signature not hex", pub, func(r *events.LambdaFunctionURLRequest) {
			r.Headers["x-signature-ed25519"] = "zzzz"
		}},
		{"signature too short", pub, func(r *events.LambdaFunctionURLRequest) {
			r.Headers["x-signature-ed25519"] = hex.EncodeToString([]byte("short"))
		}},
		{"another key", otherPub, func(*events.LambdaFunctionURLRequest) {}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := signed(t, priv, `{"type":1}`, false)
			tt.mutate(&req)

			_, err := VerifiedBody(tt.key, req)
			if !errors.Is(err, ErrInvalidSignature) {
				t.Errorf("err = %v, want ErrInvalidSignature", err)
			}
		})
	}
}

// 本文が壊れている場合は署名不正と区別する。前者は400、後者は401を返すため。
func TestVerifiedBodyRejectsMalformedBase64(t *testing.T) {
	pub, priv := keyPair(t)

	req := signed(t, priv, `{"type":1}`, true)
	req.Body = "!!! not base64 !!!"

	_, err := VerifiedBody(pub, req)
	if err == nil {
		t.Fatal("want an error")
	}
	if errors.Is(err, ErrInvalidSignature) {
		t.Errorf("err = %v, want a decode error rather than ErrInvalidSignature", err)
	}
}
