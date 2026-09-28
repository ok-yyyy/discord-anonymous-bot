package handler

import (
	"errors"
	"fmt"
)

// MsgFailed は原因を見せられない失敗のときに実行者へ返す文面。
const MsgFailed = "処理に失敗しました。時間をおいてもう一度お試しください。"

// UserError はそのまま実行者に見せてよいエラー。
//
// 内部の失敗 (権限不足、APIエラーなど) をそのまま見せると情報が漏れるうえ
// 意味も伝わらないため、見せてよい文面はこの型で明示する。
type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

func userErrorf(format string, a ...any) error {
	return &UserError{Message: fmt.Sprintf(format, a...)}
}

// UserMessage は実行者に見せる文面を返す。
//
// 見せてよいと明示されていないエラーは中身を伏せる。2つ目の戻り値は、
// 伏せたかどうか (=ログに残すべきか) を表す。
func UserMessage(err error) (string, bool) {
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.Message, true
	}
	return MsgFailed, false
}
