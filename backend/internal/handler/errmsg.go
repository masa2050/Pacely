package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/masa2050/pacely/backend/internal/service"
)

// invalidInputMessage はservice.ErrInvalidInputを含むエラーから、ユーザーに見せてよい
// 日本語の理由部分だけを取り出す(フェーズ9-1)。err.Error()をそのまま返すと
// "invalid input: ..." という英語のsentinel文言が画面に出てしまうため、その後ろだけを使う。
// 途中に別のfmt.Errorf("...: %w")でラップされていても、sentinel文言以降を取り出せる。
func invalidInputMessage(err error) string {
	_, after, ok := strings.Cut(err.Error(), service.ErrInvalidInput.Error()+": ")
	if !ok || after == "" {
		return "入力内容に誤りがあります"
	}
	return after
}

// localizedStatusMessage はEchoが自動生成する英語のエラー文("Not Found"等)の代わりに
// 返す日本語の定型文。
func localizedStatusMessage(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "リクエストが正しくありません"
	case http.StatusUnauthorized:
		return "ログインが必要です。もう一度ログインしてください"
	case http.StatusForbidden:
		return "この操作は許可されていません"
	case http.StatusNotFound:
		return "リクエスト先が見つかりません"
	case http.StatusMethodNotAllowed:
		return "この操作には対応していません"
	case http.StatusRequestEntityTooLarge:
		return "送信するデータが大きすぎます"
	case http.StatusTooManyRequests:
		return "リクエストが多すぎます。しばらくしてからお試しください"
	}
	if code >= 500 {
		return "サーバーでエラーが発生しました。しばらくしてから再度お試しください"
	}
	return "リクエストを処理できませんでした"
}

// internalError は予期しないエラーを、詳細をサーバーログにだけ残して定型文の500にする。
// 各*ToErrorのdefault分岐で同じ処理を書き写すと、新しいハンドラで書き忘れて原因不明の500に
// なりうるため一箇所にまとめている(8-1・9-1共通の「詳細はログのみ」方針)。
func internalError(tag string, err error, userMessage string) error {
	log.Printf("%s: unexpected error: %v", tag, err)
	return echo.NewHTTPError(http.StatusInternalServerError, userMessage)
}

// HTTPErrorHandler はEchoの標準エラーハンドラの前段で、エラーレスポンスのmessageを
// ユーザーに見せてよい日本語に揃える(docs/api.md 4.、docs/adr/023)。
// 各ハンドラが返す日本語のmessageはそのまま通し、次の2つだけを置き換える:
//   - Echoが自動生成する英語の定型文(未定義のルート、Recoverミドルウェアが捕まえたpanic等)
//   - echo.HTTPError以外の素のエラー(詳細はログにのみ残す)
func HTTPErrorHandler(err error, c echo.Context) {
	// 既にレスポンスを書き始めている場合はEchoの標準ハンドラと同じく何もしない。
	if c.Response().Committed {
		return
	}

	var he *echo.HTTPError
	if !errors.As(err, &he) {
		log.Printf("unhandled error: %v", err)
		he = echo.NewHTTPError(http.StatusInternalServerError)
	}
	// Echoの標準ハンドラは、Internalが*HTTPErrorならそちらを実際の返却対象にする。
	// 置換対象を取り違えないよう、こちらでも先に同じ展開をしてから文言を判定する。
	if inner, ok := he.Internal.(*echo.HTTPError); ok {
		he = inner
	}
	if msg, ok := he.Message.(string); ok && msg == http.StatusText(he.Code) {
		// echo.ErrNotFoundなどEchoのグローバル変数は全リクエストで共有されるため、
		// 直接書き換えるとデータ競合になる。コピーに対して差し替える。
		localized := *he
		localized.Message = localizedStatusMessage(he.Code)
		he = &localized
	}
	c.Echo().DefaultHTTPErrorHandler(he, c)
}
