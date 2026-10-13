import { useEffect, useState } from 'react'
import { getActiveGoalProgress, type Progress } from '../lib/api'
import { GOAL_TYPES } from './GoalForm'
import { toUserMessage } from '../lib/errors'
import { ErrorRetry } from './ErrorRetry'
import { daysUntil, formatFullDate } from '../lib/date'

const GOAL_TYPE_LABELS: Record<string, string> = Object.fromEntries(GOAL_TYPES.map((t) => [t.value, t.label]))

function formatPace(paceSecPerKm: number): string {
  const min = Math.floor(paceSecPerKm / 60)
  const sec = Math.round(paceSecPerKm % 60)
  return `${min}:${String(sec).padStart(2, '0')}/km`
}

function countdownText(targetDate: string): string {
  const days = daysUntil(targetDate)
  const dateLabel = `目標日 ${formatFullDate(targetDate)}`
  if (days > 0) return `本番まであと${days}日(${dateLabel})`
  if (days === 0) return `本日が本番です(${dateLabel})`
  return `目標日を過ぎています(${dateLabel})`
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

  // 初回はuseEffectからload()だけを呼ぶ。loading/errorの初期値が既に「読み込み中・エラー無し」
  // なので、effect内で同期的にsetStateする必要がない(lintのset-state-in-effect対策)。
  // リトライ時だけretry()で表示を「読み込み中」に戻してから取り直す(フェーズ9-4)。
  function load() {
    getActiveGoalProgress()
      .then(setProgress)
      .catch((err) => setError(toUserMessage(err)))
      .finally(() => setLoading(false))
  }

  function retry() {
    setLoading(true)
    setError(null)
    load()
  }

  useEffect(() => {
    load()
  }, [])

  if (loading) return <p>読み込み中...</p>
  if (error) return <ErrorRetry message={error} onRetry={retry} />

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
      {/* 本番(目標日)までの残り日数。progress APIが返すgoal.target_dateから計算する
          (BEの変更は不要)。練習の位置づけ(AI提案の「レースまでの残日数」と同じ情報)を
          ユーザー自身も把握できるようにする。 */}
      <p className="progress-view__countdown">{countdownText(goal.target_date)}</p>
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
