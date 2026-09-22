import { useState } from 'react'
import type { FormEvent } from 'react'
import { supabase } from '../lib/supabase'

type Props = {
  onDone: () => void
}

// パスワード再設定メールのリンクからPacelyに戻ってきたときに表示するフォーム。
// supabase.auth.onAuthStateChangeのPASSWORD_RECOVERYイベントで表示を切り替える
// (App.tsx・docs/adr/010参照)。updateUserは「今のセッションのパスワードを変える」
// APIであり、リンクのトークン自体をこちらで扱う必要はない(SDKが内部で処理する)。
export function ResetPasswordForm({ onDone }: Props) {
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)

    if (password !== confirmPassword) {
      setError('パスワードが一致しません')
      return
    }

    setSubmitting(true)
    const { error } = await supabase.auth.updateUser({ password })
    setSubmitting(false)

    if (error) {
      setError(error.message)
      return
    }
    onDone()
  }

  return (
    <form className="auth-form" onSubmit={handleSubmit}>
      <h1>Pacely</h1>
      <p className="auth-form__subtitle">新しいパスワードを設定</p>

      <label>
        新しいパスワード
        <input
          type="password"
          required
          minLength={6}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
        />
      </label>

      <label>
        新しいパスワード(確認)
        <input
          type="password"
          required
          minLength={6}
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          autoComplete="new-password"
        />
      </label>

      {error && <p className="auth-form__message">{error}</p>}

      <button type="submit" disabled={submitting}>
        {submitting ? '更新中...' : 'パスワードを更新する'}
      </button>
    </form>
  )
}
