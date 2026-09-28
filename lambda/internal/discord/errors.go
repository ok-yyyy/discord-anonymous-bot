package discord

import (
	"errors"
	"net/http"

	"github.com/disgoorg/disgo/rest"
)

// IsForbidden はDiscordが権限不足で拒否したかを返す。
func IsForbidden(err error) bool {
	return hasStatus(err, http.StatusForbidden)
}

// IsNotFound は対象が存在しないことを表すかを返す。
// Webhookやメッセージが削除されている場合の判定に使う。
func IsNotFound(err error) bool {
	return hasStatus(err, http.StatusNotFound)
}

// hasStatus はdisgoのエラーから状態コードを取り出して比較する。
// rest.Errorはポインタで返るため、値で受けるとerrors.Asが一致しない。
func hasStatus(err error, status int) bool {
	var restErr *rest.Error
	return errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == status
}
