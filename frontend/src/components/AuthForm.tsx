import { useState } from 'react'
import type { FormEvent } from 'react'
import { supabase } from '../lib/supabase'

type Mode = 'login' | 'signup' | 'reset-request'

// サインアップ・ログイン・パスワード再設定依頼はSupabase Auth SDKをフロントから
// 直接呼ぶ設計(docs/api.md 2.1)。Pacelyバックエンドには認証専用エンドポイントを作らない。
export function AuthForm() {
  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [username, setUsername] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setMessage(null)
    setSubmitting(true)

    if (mode === 'reset-request') {
      // redirectTo に window.location.origin を使うことで、ローカル(localhost:5173)・
      // 本番(Vercelドメイン)どちらでもコード変更なしにリンク先を切り替えられる。
      // Supabase側の Authentication > URL Configuration の Redirect URLs に
      // 両方のオリジンを許可リスト登録しておく必要がある(docs/adr/010参照)。
      const { error } = await supabase.auth.resetPasswordForEmail(email, {
        redirectTo: window.location.origin,
      })
      setSubmitting(false)
      if (error) {
        setMessage(error.message)
        return
      }
      setMessage('パスワード再設定用のメールを送信しました。メール内のリンクから新しいパスワードを設定してください。')
      return
    }

    // usernameはPacely独自のusersテーブルのカラムで、Supabase Authそのものは知らない値だが、
    // options.dataに渡すとuser_metadataとしてJWTに含まれるようになる。バックエンドの
    // GET /users/me(初回アクセス時)がそこから読み取ってusersテーブルの行作成時に使う
    // (docs/adr/017)。"Confirm email" が有効な本番環境でも、確認前のsignUp時点で
    // Supabase Auth側に保存されるため、確認メールのリンクを踏んで実際にログインするまで
    // 時間が空いても値は失われない。
    const { error } =
      mode === 'login'
        ? await supabase.auth.signInWithPassword({ email, password })
        : await supabase.auth.signUp({ email, password, options: { data: { username } } })

    setSubmitting(false)

    if (error) {
      setMessage(error.message)
      return
    }
    if (mode === 'signup') {
      setMessage('登録しました。そのままログインできます。')
    }
    // ログイン成功時は onAuthStateChange 経由で useSession が自動的に更新される。
  }

  return (
    <form className="auth-form" onSubmit={handleSubmit}>
      <h1>Pacely</h1>
      <p className="auth-form__subtitle">
        {mode === 'login' ? 'ログイン' : mode === 'signup' ? '新規登録' : 'パスワード再設定'}
      </p>

      <label>
        メールアドレス
        <input
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="email"
        />
      </label>

      {mode !== 'reset-request' && (
        <label>
          パスワード
          <input
            type="password"
            required
            minLength={6}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          />
        </label>
      )}

      {mode === 'signup' && (
        <label>
          ユーザーネーム(画面上の表示名)
          <input
            required
            maxLength={50}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
          />
        </label>
      )}

      {message && <p className="auth-form__message">{message}</p>}

      <button type="submit" disabled={submitting}>
        {submitting
          ? '処理中...'
          : mode === 'login'
            ? 'ログイン'
            : mode === 'signup'
              ? '登録する'
              : '再設定メールを送信する'}
      </button>

      {mode === 'login' && (
        <button
          type="button"
          className="auth-form__switch"
          onClick={() => {
            setMode('reset-request')
            setMessage(null)
          }}
        >
          パスワードをお忘れですか?
        </button>
      )}

      <button
        type="button"
        className="auth-form__switch"
        onClick={() => {
          setMode(mode === 'login' ? 'signup' : 'login')
          setMessage(null)
        }}
      >
        {mode === 'login' ? 'アカウントを新規作成する' : 'ログイン画面に戻る'}
      </button>
    </form>
  )
}
