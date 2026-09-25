import { useEffect, useState } from 'react'
import {
  getLatestAdvice,
  listAdvices,
  submitAdviceFeedback,
  type Advice,
  type LatestAdviceResult,
} from '../lib/api'

function formatPace(paceSecPerKm: number): string {
  const min = Math.floor(paceSecPerKm / 60)
  const sec = Math.round(paceSecPerKm % 60)
  return `${min}:${String(sec).padStart(2, '0')}/km`
}

// 合計時間(distance_km * pace_sec_per_km)を表示用に整形する。
// RunForm(時間入力欄)と同様「時間/分/秒」の単位表記に合わせる。
function formatDuration(totalSec: number): string {
  const sec = Math.round(totalSec)
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}時間${m}分`
  if (m > 0) return s > 0 ? `${m}分${s}秒` : `${m}分`
  return `${s}秒`
}

// フェーズ7-5: 表示中のアドバイスへのフィードバック(docs/adr/019)。
// 評価ボタンを押した時点で、その時テキストエリアに入っているコメントも一緒に送る。
// こうすると「評価」と「コメント」で2回APIを叩かずに済み、押し直せば上書きもできる
// (PUTなので冪等。未評価に戻す操作は今回のスコープ外)。
function AdviceFeedback({ advice, onSaved }: { advice: Advice; onSaved: (updated: Advice) => void }) {
  const [comment, setComment] = useState(advice.feedback_comment ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function save(isHelpful: boolean) {
    setSaving(true)
    setError(null)
    try {
      onSaved(await submitAdviceFeedback(advice.id, isHelpful, comment))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="advice-feedback">
      <p className="advice-feedback__label">この提案は役に立ちましたか?</p>
      {/* テキストボタン(「役に立った」「役に立たなかった」)を横並びにすると
          文字量の差で見た目のバランスが悪く分かりにくかったため、意味が
          直感的に伝わるグッドマーク・バッドマークのアイコンボタンに変更した。
          aria-labelで意味を明示し、スクリーンリーダーでも区別できるようにする。 */}
      <div className="advice-feedback__buttons">
        <button
          type="button"
          className={`advice-feedback__button advice-feedback__button--good${advice.is_helpful === true ? ' advice-feedback__button--selected' : ''}`}
          onClick={() => save(true)}
          disabled={saving}
          aria-label="役に立った"
          aria-pressed={advice.is_helpful === true}
          title="役に立った"
        >
          👍
        </button>
        <button
          type="button"
          className={`advice-feedback__button advice-feedback__button--bad${advice.is_helpful === false ? ' advice-feedback__button--selected' : ''}`}
          onClick={() => save(false)}
          disabled={saving}
          aria-label="役に立たなかった"
          aria-pressed={advice.is_helpful === false}
          title="役に立たなかった"
        >
          👎
        </button>
      </div>

      <label className="advice-feedback__comment-label">
        コメント(任意)
        <textarea
          className="advice-feedback__comment"
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          maxLength={500}
          rows={2}
          placeholder="例: ペースが少しきつかった"
          disabled={saving}
        />
      </label>

      {advice.feedback_at && (
        <p className="advice-feedback__saved">
          送信済み({new Date(advice.feedback_at).toLocaleString('ja-JP')})。ボタンを押し直すと上書きされます。
        </p>
      )}
      {error && <p className="dashboard__error">{error}</p>}
    </div>
  )
}

function AdviceCard({ advice, onFeedbackSaved }: { advice: Advice; onFeedbackSaved: (updated: Advice) => void }) {
  const { distance_km, pace_sec_per_km, note } = advice.next_menu
  return (
    <div className="advice-view__card">
      <p className="advice-view__text">{advice.advice_text}</p>

      {/* 「次回」だと誤解を招く(このメニューはその日の24時間キャッシュ対象、docs/adr/004)ため
          「本日の練習メニュー」と表記する。距離・ペース・合計時間はdt/ddの縦積みではなく
          横並びの統計表示にして、狭い画面でも縦に間延びしないようにする。 */}
      <div className="advice-view__menu">
        <p className="advice-view__menu-label">本日の練習メニュー</p>
        <div className="advice-view__stats">
          <div className="advice-view__stat">
            <span className="advice-view__stat-label">距離</span>
            <span className="advice-view__stat-value">{distance_km}km</span>
          </div>
          <div className="advice-view__stat">
            <span className="advice-view__stat-label">ペース</span>
            <span className="advice-view__stat-value">{formatPace(pace_sec_per_km)}</span>
          </div>
          <div className="advice-view__stat">
            <span className="advice-view__stat-label">合計時間</span>
            <span className="advice-view__stat-value">{formatDuration(distance_km * pace_sec_per_km)}</span>
          </div>
        </div>
        {note && <p className="advice-view__note">{note}</p>}
      </div>

      {advice.weather_context.has_weather && (
        <p className="advice-view__weather">
          生成時の天候: {advice.weather_context.description} / 気温{advice.weather_context.temp_c}℃
        </p>
      )}
      <p className="advice-view__generated-at">生成日時: {new Date(advice.generated_at).toLocaleString('ja-JP')}</p>

      <AdviceFeedback advice={advice} onSaved={onFeedbackSaved} />
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

  // 同じadviceが「最新」と「履歴」の両方に出ることがあるため、保存後のレコードを
  // 両方に反映して表示がずれないようにする。
  function applyFeedback(updated: Advice) {
    setResult((prev) => (prev?.advice?.id === updated.id ? { ...prev, advice: updated } : prev))
    setHistory((prev) => prev.map((a) => (a.id === updated.id ? updated : a)))
  }

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
      {/* keyにadvice.idを渡すことで、新しい提案が生成された際にコメント入力欄の
          state(前の提案に対する入力)が残らないようにする。 */}
      {result.status === 'ready' && result.advice && (
        <AdviceCard key={result.advice.id} advice={result.advice} onFeedbackSaved={applyFeedback} />
      )}

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
              <AdviceCard advice={advice} onFeedbackSaved={applyFeedback} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
