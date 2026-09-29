import { useEffect, useState } from 'react'
import { getLatestAdvice, type Advice, type LatestAdviceResult } from '../lib/api'
import { AdviceCard } from './AdviceCard'

type Props = {
  onHistoryClick: () => void
}

// フェーズ5: GET /advices/latest の結果を表示する画面。
// このエンドポイントは呼び出し時に「記録3件未満」「目標未設定」「前回生成から24時間
// 未経過」を判定し、必要な場合のみAIを呼び出す(docs/api.md 3.、docs/adr/004)。
// つまりGETなのに副作用(AI呼び出し・DB書き込み)を持つ意図的な設計であり、
// このコンポーネントをマウントするたびにAI生成が走るわけではない
// (キャッシュがあればキャッシュを返すだけ)。
//
// フェーズ9-5(docs/adr/020)で履歴表示の責務を外し、ここは「本日の提案」表示に
// 専念する。以前はこのカード内で過去の提案をAdviceCardのまま縦に積んでいたため、
// 件数が増えるほどダッシュボードが下へ伸びていた。
export function AdviceView({ onHistoryClick }: Props) {
  const [result, setResult] = useState<LatestAdviceResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  function fetchLatest() {
    setLoading(true)
    setError(null)
    getLatestAdvice()
      .then(setResult)
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchLatest()
  }, [])

  function applyFeedback(updated: Advice) {
    setResult((prev) => (prev?.advice?.id === updated.id ? { ...prev, advice: updated } : prev))
  }

  if (loading) return <p>AI提案を確認中...</p>
  if (error) return <p className="dashboard__error">{error}</p>
  if (!result) return null

  return (
    <div className="advice-view">
      {result.status === 'needs_more_runs' && (
        <p>
          AI提案にはあと{Math.max(result.min_runs_required - result.runs_count, 0)}件の記録が必要です(現在{result.runs_count}
          /{result.min_runs_required}件)。まずは記録を追加してください。
        </p>
      )}
      {result.status === 'needs_active_goal' && <p>AI提案には目標の設定が必要です。まず目標を設定してください。</p>}
      {/* keyにadvice.idを渡すことで、新しい提案が生成された際にコメント入力欄の
          state(前の提案に対する入力)が残らないようにする。 */}
      {result.status === 'ready' && result.advice && (
        <AdviceCard key={result.advice.id} advice={result.advice} onFeedbackSaved={applyFeedback} />
      )}

      <div className="advice-view__actions">
        <button type="button" onClick={fetchLatest}>
          最新のAI提案を確認する
        </button>
        {/* カード内での開閉(showHistory)をやめ、専用画面への導線に変更した
            (docs/adr/020)。履歴は「振り返りたいときに開く情報」であり、
            ダッシュボードに常駐させる必要がないため。 */}
        <button type="button" onClick={onHistoryClick}>
          過去の提案履歴を見る
        </button>
      </div>
    </div>
  )
}
