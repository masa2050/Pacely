import { useEffect, useState } from 'react'
import type { Session } from '@supabase/supabase-js'
import {
  getMe,
  updateMyRegion,
  deleteAccount,
  listRuns,
  listGoals,
  getActiveGoal,
  type Me,
  type Run,
  type Goal,
} from '../lib/api'
import { supabase } from '../lib/supabase'
import { getGreeting } from '../lib/greeting'
import { RunForm } from './RunForm'
import { RunList } from './RunList'
import { GoalForm } from './GoalForm'
import { GoalList } from './GoalList'
import { ProgressView } from './ProgressView'
import { AdviceView } from './AdviceView'

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
  const [runs, setRuns] = useState<Run[]>([])
  const [runsError, setRunsError] = useState<string | null>(null)
  const [goals, setGoals] = useState<Goal[]>([])
  const [activeGoal, setActiveGoal] = useState<Goal | null>(null)
  const [goalsError, setGoalsError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    getMe()
      .then((data) => {
        setMe(data)
        setRegion(data.region ?? '')
      })
      .catch((err) => setError(err.message))

    listRuns()
      .then(setRuns)
      .catch((err) => setRunsError(err.message))

    listGoals()
      .then(setGoals)
      .catch((err) => setGoalsError(err.message))

    getActiveGoal()
      .then(setActiveGoal)
      .catch((err) => setGoalsError(err.message))
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
    <div className="dashboard">
      <header className="dashboard__header">
        <h1>Pacely</h1>
        <button type="button" onClick={() => supabase.auth.signOut()}>
          ログアウト
        </button>
      </header>

      <p className="dashboard__greeting">
        {getGreeting()}
        {session.user.email ? `、${session.user.email.split('@')[0]}さん` : ''}
      </p>
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

      <div className="dashboard__card">
        <h2>ランニング記録</h2>
        {runsError && <p className="dashboard__error">{runsError}</p>}
        <RunForm onSaved={(run) => setRuns((prev) => [run, ...prev])} />
        <RunList runs={runs} onChanged={setRuns} />
      </div>

      <div className="dashboard__card">
        <h2>目標設定</h2>
        {goalsError && <p className="dashboard__error">{goalsError}</p>}
        <GoalList
          activeGoal={activeGoal}
          goals={goals}
          onActiveChanged={setActiveGoal}
          onGoalsChanged={setGoals}
        />
        <GoalForm
          onSaved={(goal) => {
            // 新規作成時、既存のactive目標はBE側で自動的にabandonedへ更新される
            // (docs/adr/009)。ローカル側の反映漏れを防ぐため一覧ごと取り直す。
            setActiveGoal(goal)
            listGoals()
              .then(setGoals)
              .catch((err) => setGoalsError(err.message))
          }}
        />
      </div>

      <div className="dashboard__card">
        <h2>進捗確認</h2>
        {/* activeGoalの切り替わりやrunsの増減のたびに進捗を取り直したいので、
            それらをkeyにしてコンポーネントごと再マウントする(簡易的な再取得トリガー)。 */}
        <ProgressView key={`${activeGoal?.id ?? 'none'}-${runs.length}`} />
      </div>

      <div className="dashboard__card">
        <h2>AI提案(GET /advices/latest)</h2>
        {/* activeGoal・runsの変化に応じて再取得したいので、Progressと同様keyで再マウントする。
            ただし取得自体は「前回生成から24時間以内ならキャッシュを返す」ため、
            毎回AIが呼ばれるわけではない(docs/adr/004)。 */}
        <AdviceView key={`${activeGoal?.id ?? 'none'}-${runs.length}`} />
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
