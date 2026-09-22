import { useEffect, useState } from 'react'
import { getActiveGoalProgress, type Progress } from '../lib/api'
import { GOAL_TYPES } from './GoalForm'

const GOAL_TYPE_LABELS: Record<string, string> = Object.fromEntries(GOAL_TYPES.map((t) => [t.value, t.label]))

function formatPace(paceSecPerKm: number): string {
  const min = Math.floor(paceSecPerKm / 60)
  const sec = Math.round(paceSecPerKm % 60)
  return `${min}:${String(sec).padStart(2, '0')}/km`
}

// diffは「平均ペース - 目標ペース」(秒/km)。正なら目標より遅い、負なら速い。
function formatDiff(diffSecPerKm: number): string {
  const sec = Math.round(Math.abs(diffSecPerKm))
  if (sec === 0) return 'ちょうど目標ペースです'
  return diffSecPerKm > 0 ? `目標より ${sec}秒/km 遅いペースです` : `目標より ${sec}秒/km 速いペースです`
}

// フェーズ4: GET /goals/active/progress の結果を表示するだけの画面。
// 平均ペース・目標ペースとの差分はバックエンド(service層)で計算済みのものを
// そのまま表示する(docs/architecture.md 3.: 差分計算はservice層の責務)。
export function ProgressView() {
  const [progress, setProgress] = useState<Progress | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getActiveGoalProgress()
      .then(setProgress)
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p>読み込み中...</p>
  if (error) return <p className="dashboard__error">{error}</p>

  if (!progress) {
    return <p>現在有効な目標はありません。目標を設定すると進捗が表示されます。</p>
  }

  const { goal, target_pace_sec_per_km, recent_average_pace_sec_per_km, recent_runs_count, pace_diff_sec_per_km } =
    progress

  return (
    <div className="progress-view">
      <p className="progress-view__goal">
        {GOAL_TYPE_LABELS[goal.goal_type] ?? goal.goal_type} / 目標ペース {formatPace(target_pace_sec_per_km)}
      </p>
      {recent_runs_count === 0 || recent_average_pace_sec_per_km === null ? (
        <p>まだ記録がありません。記録を追加すると進捗が表示されます。</p>
      ) : (
        <dl>
          <dt>直近{recent_runs_count}件の平均ペース</dt>
          <dd>{formatPace(recent_average_pace_sec_per_km)}</dd>
          <dt>目標との差</dt>
          <dd>{formatDiff(pace_diff_sec_per_km ?? 0)}</dd>
        </dl>
      )}
    </div>
  )
}
