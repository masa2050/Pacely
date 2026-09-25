import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import type { Session } from '@supabase/supabase-js'
import { getMe, updateMyProfile, changePassword, deleteAccount, type Me } from '../lib/api'
import { supabase } from '../lib/supabase'
import { PREFECTURES } from '../lib/prefectures'

type Props = {
  session: Session
  onBack: () => void
}

// フェーズ7-2: 設定画面。プロフィール編集(地域・ユーザーネーム)と退会をここにまとめる。
// フェーズ7-3でusernameカラムが追加されたため、region専用だった保存フォームを
// region・username共通のプロフィールフォームに拡張した。
// 退会は元々Dashboardの一番下にあり目立たない位置だったため、専用画面に移動した
// (docs/implementation-plan.md 7-2)。移動に伴いDashboard.tsxのプロフィールカードは
// 表示専用にし、編集操作はこの画面に一本化した(同じ編集フォームを2箇所に持たないため)。
export function Settings({ session, onBack }: Props) {
  const [me, setMe] = useState<Me | null>(null)
  const [region, setRegion] = useState('')
  const [username, setUsername] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmNewPassword, setConfirmNewPassword] = useState('')
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)
  const [changingPassword, setChangingPassword] = useState(false)

  useEffect(() => {
    getMe()
      .then((data) => {
        setMe(data)
        setRegion(data.region ?? '')
        setUsername(data.username ?? '')
      })
      .catch((err) => setError(err.message))
  }, [])

  // 未入力のフィールドはリクエストに含めない。バックエンドは空文字を400で弾くため
  // (「片方だけ更新」をボディにフィールドが有るかどうかで判定している、docs/adr/017)、
  // 例えば地域未設定のユーザーがユーザーネームだけ保存するケースで失敗しないようにする。
  async function handleSaveProfile() {
    const input: { region?: string; username?: string } = {}
    if (region) input.region = region
    if (username) input.username = username
    if (!input.region && !input.username) {
      setError('ユーザーネームか地域のいずれかを入力してください')
      return
    }

    setSaving(true)
    setError(null)
    try {
      const updated = await updateMyProfile(input)
      setMe(updated)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  // パスワード変更(フェーズ7-4)。バックエンドの/users/me/passwordは「現在のパスワードが
  // 正しいこと」自体は検証しないため、先にsignInWithPasswordで再認証してから呼び出す
  // (docs/adr/016)。再認証に失敗した場合はバックエンドを呼ばない。
  async function handleChangePassword(e: FormEvent) {
    e.preventDefault()
    setPasswordError(null)
    setPasswordMessage(null)

    if (!session.user.email) {
      setPasswordError('メールアドレスが取得できませんでした')
      return
    }
    if (newPassword !== confirmNewPassword) {
      setPasswordError('新しいパスワードが一致しません')
      return
    }

    setChangingPassword(true)
    try {
      const { error: reauthError } = await supabase.auth.signInWithPassword({
        email: session.user.email,
        password: currentPassword,
      })
      if (reauthError) {
        setPasswordError('現在のパスワードが正しくありません')
        return
      }

      await changePassword(newPassword)
      setPasswordMessage('パスワードを変更しました')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmNewPassword('')
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : String(err))
    } finally {
      setChangingPassword(false)
    }
  }

  // 退会(docs/adr/014)。取り消せない操作なので、確認ダイアログを必ず挟む。
  // 削除完了後はサーバー側でSupabase Authアカウントごと消えているため、
  // フロントもsignOutしてログイン画面に戻す。
  async function handleDeleteAccount() {
    if (!window.confirm('退会すると、記録・目標・AI提案の履歴がすべて削除され、元に戻せません。本当に退会しますか?')) {
      return
    }
    setDeleting(true)
    setError(null)
    try {
      await deleteAccount()
      await supabase.auth.signOut()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setDeleting(false)
    }
  }

  return (
    <div className="settings">
      <button type="button" className="settings__back" onClick={onBack}>
        ← 戻る
      </button>

      <h1>設定</h1>

      {error && <p className="dashboard__error">{error}</p>}

      <div className="dashboard__card">
        <h2>プロフィール</h2>
        {me ? (
          <>
            <dl>
              <dt>user id</dt>
              <dd>{me.id}</dd>
            </dl>

            <div className="dashboard__region-form">
              <label>
                ユーザーネーム(表示名)
                <input
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="例: taro"
                  maxLength={50}
                />
              </label>
              <label>
                地域(天候取得用)
                <select value={region} onChange={(e) => setRegion(e.target.value)}>
                  <option value="">(未設定)</option>
                  {/* フェーズ7-3以前に自由入力で保存された値(例: "Tokyo")は選択肢に無いため、
                      そのままでは「未設定」に見えてしまう。現在の保存値を選択肢として足しておく。 */}
                  {region && !PREFECTURES.includes(region as (typeof PREFECTURES)[number]) && (
                    <option value={region}>{region}(以前の設定)</option>
                  )}
                  {PREFECTURES.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </label>
              <button type="button" onClick={handleSaveProfile} disabled={saving}>
                {saving ? '保存中...' : '保存(PUT /users/me)'}
              </button>
            </div>
          </>
        ) : (
          !error && <p>読み込み中...</p>
        )}
      </div>

      <div className="dashboard__card">
        <h2>パスワード変更</h2>
        <form className="settings__password-form" onSubmit={handleChangePassword}>
          <label>
            現在のパスワード
            <input
              type="password"
              required
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              autoComplete="current-password"
            />
          </label>
          <label>
            新しいパスワード
            <input
              type="password"
              required
              minLength={6}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              autoComplete="new-password"
            />
          </label>
          <label>
            新しいパスワード(確認)
            <input
              type="password"
              required
              minLength={6}
              value={confirmNewPassword}
              onChange={(e) => setConfirmNewPassword(e.target.value)}
              autoComplete="new-password"
            />
          </label>

          {passwordError && <p className="dashboard__error">{passwordError}</p>}
          {passwordMessage && <p className="settings__message">{passwordMessage}</p>}

          <button type="submit" disabled={changingPassword}>
            {changingPassword ? '変更中...' : 'パスワードを変更する'}
          </button>
        </form>
      </div>

      <div className="dashboard__card dashboard__card--danger">
        <h2>退会</h2>
        <p>退会すると、記録・目標・AI提案の履歴を含むすべてのデータが削除され、元に戻せません。</p>
        <button type="button" onClick={handleDeleteAccount} disabled={deleting}>
          {deleting ? '処理中...' : '退会する(DELETE /users/me)'}
        </button>
      </div>
    </div>
  )
}
