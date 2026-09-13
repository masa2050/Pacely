import { useEffect, useState } from 'react'
import type { Session } from '@supabase/supabase-js'
import { getMe, updateMyRegion, type Me } from '../lib/api'
import { supabase } from '../lib/supabase'

type Props = {
  session: Session
}

// フェーズ1の完了条件確認用: ログイン後、JWTを使って GET/PUT /users/me が
// 実際に動くことをUI上で見えるようにする最小限の画面。
export function Dashboard({ session }: Props) {
  const [me, setMe] = useState<Me | null>(null)
  const [region, setRegion] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

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

  return (
    <div className="dashboard">
      <header className="dashboard__header">
        <h1>Pacely</h1>
        <button type="button" onClick={() => supabase.auth.signOut()}>
          ログアウト
        </button>
      </header>

      <p className="dashboard__email">{session.user.email}</p>

      {error && <p className="dashboard__error">{error}</p>}

      {me ? (
        <div className="dashboard__card">
          <h2>プロフィール(GET /users/me)</h2>
          <dl>
            <dt>user id</dt>
            <dd>{me.id}</dd>
            <dt>region</dt>
            <dd>{me.region ?? '(未設定)'}</dd>
            <dt>created_at</dt>
            <dd>{me.created_at}</dd>
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
        </div>
      ) : (
        !error && <p>読み込み中...</p>
      )}
    </div>
  )
}
