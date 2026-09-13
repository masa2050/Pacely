import { useState } from 'react'
import type { FormEvent } from 'react'
import { supabase } from '../lib/supabase'

type Mode = 'login' | 'signup'

// サインアップ・ログインはSupabase Auth SDKをフロントから直接呼ぶ設計(docs/api.md 2.1)。
// Pacelyバックエンドには認証専用エンドポイントを作らない。
export function AuthForm() {
  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setMessage(null)
    setSubmitting(true)

    const { error } =
      mode === 'login'
        ? await supabase.auth.signInWithPassword({ email, password })
        : await supabase.auth.signUp({ email, password })

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
        {mode === 'login' ? 'ログイン' : '新規登録'}
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

      {message && <p className="auth-form__message">{message}</p>}

      <button type="submit" disabled={submitting}>
        {submitting ? '処理中...' : mode === 'login' ? 'ログイン' : '登録する'}
      </button>

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
