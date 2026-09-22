import { useEffect, useState } from 'react'
import { getLatestAdvice, listAdvices, type Advice, type LatestAdviceResult } from '../lib/api'

function formatPace(paceSecPerKm: number): string {
  const min = Math.floor(paceSecPerKm / 60)
  const sec = Math.round(paceSecPerKm % 60)
  return `${min}:${String(sec).padStart(2, '0')}/km`
}

function AdviceCard({ advice }: { advice: Advice }) {
  return (
    <div className="advice-view__card">
      <p className="advice-view__text">{advice.advice_text}</p>
      <dl>
        <dt>次回練習メニュー</dt>
        <dd>
          {advice.next_menu.distance_km}km / {formatPace(advice.next_menu.pace_sec_per_km)}
        </dd>
        {advice.next_menu.note && <dd className="advice-view__note">{advice.next_menu.note}</dd>}
      </dl>
      {advice.weather_context.has_weather && (
        <p className="advice-view__weather">
          生成時の天候: {advice.weather_context.description} / 気温{advice.weather_context.temp_c}℃
        </p>
      )}
      <p className="advice-view__generated-at">生成日時: {new Date(advice.generated_at).toLocaleString('ja-JP')}</p>
    </div>
  )
}

// フェーズ5: GET /advices/latest の結果を表示する画面。
// このエンドポイントは呼び出し時に「記録3件未満」「目標未設定」「前回生成から24時間
// 未経過」を判定し、必要な場合のみAIを呼び出す(docs/api.md 3.、docs/adr/004)。
// つまりGETなのに副作用(AI呼び出し・DB書き込み)を持つ意図的な設計であり、
// このコンポーネントをマウントするたびにAI生成が走るわけではない
// (キャッシュがあればキャッシュを返すだけ)。
export function AdviceView() {
  const [result, setResult] = useState<LatestAdviceResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [history, setHistory] = useState<Advice[]>([])
  const [showHistory, setShowHistory] = useState(false)

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

  function handleShowHistory() {
    setShowHistory((prev) => !prev)
    if (!showHistory && history.length === 0) {
      listAdvices()
        .then(setHistory)
        .catch((err) => setError(err instanceof Error ? err.message : String(err)))
    }
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
      {result.status === 'ready' && result.advice && <AdviceCard advice={result.advice} />}

      <div className="advice-view__actions">
        <button type="button" onClick={fetchLatest}>
          最新のAI提案を確認する(GET /advices/latest)
        </button>
        <button type="button" onClick={handleShowHistory}>
          {showHistory ? '履歴を閉じる' : '過去の提案履歴を見る'}
        </button>
      </div>

      {showHistory && (
        <ul className="advice-view__history">
          {history.length === 0 && <li>過去の提案はまだありません。</li>}
          {history.map((advice) => (
            <li key={advice.id}>
              <AdviceCard advice={advice} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
