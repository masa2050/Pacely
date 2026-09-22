import { useEffect, useState } from 'react'
import { getMe, updateMyRegion, deleteAccount, type Me } from '../lib/api'
import { supabase } from '../lib/supabase'

type Props = {
  onBack: () => void
}

// フェーズ7-2: 設定画面。
// プロフィール編集(地域)と退会をここにまとめる。ユーザーネームの編集は
// usersテーブルにusernameカラムが無くまだ実装できないため、7-3で対応する。
// 退会は元々Dashboardの一番下にあり目立たない位置だったため、専用画面に移動した
// (docs/implementation-plan.md 7-2)。移動に伴いDashboard.tsxのプロフィールカードは
// 表示専用にし、編集操作はこの画面に一本化した(同じ編集フォームを2箇所に持たないため)。
export function Settings({ onBack }: Props) {
  const [me, setMe] = useState<Me | null>(null)
  const [region, setRegion] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    getMe()
      .then((data) => {
        setMe(data)
        setRegion(data.region ?? '')
      })
      .catch((err) => setError(err.message))
  }, [])

  async function handleSaveRegion() {
    setSaving(true)
    setError(null)
    try {
      const updated = await updateMyRegion(region)
      setMe(updated)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
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
              <dt>region</dt>
              <dd>{me.region ?? '(未設定)'}</dd>
            </dl>

            <div className="dashboard__region-form">
              <label>
                地域(天候取得用)
                <input
                  value={region}
                  onChange={(e) => setRegion(e.target.value)}
                  placeholder="例: Tokyo"
                />
              </label>
              <button type="button" onClick={handleSaveRegion} disabled={saving}>
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
        <p className="settings__placeholder">近日対応予定です。</p>
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
