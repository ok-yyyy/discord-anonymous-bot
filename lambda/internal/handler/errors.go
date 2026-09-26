package handler

import "fmt"

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
