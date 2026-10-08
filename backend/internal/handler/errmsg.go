package handler

import (
	"strings"

	"github.com/masa2050/pacely/backend/internal/service"
)

// invalidInputMessage はservice.ErrInvalidInputでラップされたエラーから、
// ユーザーに見せてよい日本語の理由部分だけを取り出す(フェーズ9-1)。
// err.Error()をそのまま返すと "invalid input: ..." という英語のsentinel文言の
// プレフィックスが画面に出てしまうため、取り除く。
func invalidInputMessage(err error) string {
	return strings.TrimPrefix(err.Error(), service.ErrInvalidInput.Error()+": ")
}
