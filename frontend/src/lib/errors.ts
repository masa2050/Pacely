import { isAuthError } from '@supabase/supabase-js'
import { ApiError } from './api'

// フェーズ9-1: 画面に出すエラー文を日本語に統一するための変換。
// 以前は各コンポーネントが err.message をそのまま表示していたため、
// Supabase Authの英語メッセージ("Invalid login credentials"等)や
// ネットワーク断の "Failed to fetch" がそのままユーザーに見えていた。
// ここを通すと、想定外のエラーでも英語の生メッセージ・内部情報は画面に出ず、
// 定型の日本語になる(docs/implementation-plan.md 8-1と同じ考え方)。

const GENERIC_MESSAGE = '処理に失敗しました。時間をおいてもう一度お試しください'
const SERVER_MESSAGE = 'サーバーが混み合っています。しばらくしてからもう一度お試しください'
const NETWORK_MESSAGE = 'サーバーに接続できませんでした。通信環境を確認して、もう一度お試しください'

export const DUPLICATE_EMAIL_MESSAGE =
  'このメールアドレスは既に登録されています。ログインするか、パスワードをお忘れの場合は再設定してください'

// Supabase Authが返すerror.code(auth-jsのErrorCode)に対応する日本語。
// 未掲載のcodeは汎用メッセージに落とす(英語の生メッセージは出さない)。
const AUTH_ERROR_MESSAGES: Record<string, string> = {
  invalid_credentials: 'メールアドレスまたはパスワードが正しくありません',
  user_already_exists: DUPLICATE_EMAIL_MESSAGE,
  email_exists: DUPLICATE_EMAIL_MESSAGE,
  weak_password: 'パスワードが簡単すぎます。より推測されにくいパスワードを設定してください',
  same_password: '現在と同じパスワードは設定できません',
  email_not_confirmed: 'メールアドレスの確認が完了していません。確認メールのリンクを開いてください',
  validation_failed: '入力内容を確認して、もう一度お試しください',
  otp_expired: 'リンクの有効期限が切れています。もう一度最初からやり直してください',
  email_address_invalid: 'メールアドレスの形式が正しくありません',
  signup_disabled: '現在、新規登録を受け付けていません',
  over_email_send_rate_limit: 'メールの送信回数が上限に達しました。しばらくしてからお試しください',
  over_request_rate_limit: '操作の回数が上限に達しました。しばらくしてからお試しください',
}

export function toUserMessage(err: unknown): string {
  // バックエンド(Pacely API)のmessageは日本語の定型文で返す設計(docs/api.md 4.)なのでそのまま使う。
  if (err instanceof ApiError) return err.message

  if (isAuthError(err)) {
    // status 0 はSupabase SDKがfetch自体に失敗した場合(AuthRetryableFetchError)。
    if (err.status === 0) return NETWORK_MESSAGE
    // 502/503/504等。リトライで直る可能性があるのでcode無しの汎用文とは分ける。
    if (err.status !== undefined && err.status >= 500) return SERVER_MESSAGE
    return (err.code && AUTH_ERROR_MESSAGES[err.code]) || GENERIC_MESSAGE
  }

  // fetchがネットワーク断で投げるのはTypeError("Failed to fetch")。
  if (err instanceof TypeError) return NETWORK_MESSAGE

  return GENERIC_MESSAGE
}
