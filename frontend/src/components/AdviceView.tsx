import { useEffect, useState } from 'react'
import { getLatestAdvice, type Advice, type LatestAdviceResult } from '../lib/api'
import { AdviceCard } from './AdviceCard'
import { toUserMessage } from '../lib/errors'
import { ErrorRetry } from './ErrorRetry'

// キャッシュ(24時間以内の生成済み提案)を返すだけなら通常1秒以内に返る一方、
// 新規生成はAI API呼び出しを含み実測で10〜30秒、タイムアウト時は天候API(10秒)+AI API(60秒)で
// 最大約70秒かかる。
// この閾値を超えても返ってこない場合は「新規生成中の可能性が高い」とみなして表示を切り替える。
const SLOW_THRESHOLD_MS = 3000

type Props = {
  onHistoryClick: () => void
  // 記録件数・目標の面でAI提案を生成できる状態か(Dashboardが判定)。falseのときは
  // 遅くても「生成しています」とは出さない(needs_more_runs等で生成されないのに嘘になるため)。
  canGenerate: boolean
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
export function AdviceView({ onHistoryClick, canGenerate }: Props) {
  const [result, setResult] = useState<LatestAdviceResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // 読み込みがSLOW_THRESHOLD_MSを超えたか(フェーズ9-4)。待っている間、キャッシュ返却か
  // 新規生成かはAPIの応答が返るまで分からない(docs/api.md 3.の判定はBE側で行う)ため、
  // 経過時間から推測して文言を切り替える。APIにフラグを足す案はBE変更とADRが要る割に、
  // 「待っている最中」の表示には使えないため採用しなかった。
  const [slow, setSlow] = useState(false)

  // 初回はuseEffectからload()だけを呼ぶ。loading/errorの初期値が既に「読み込み中・エラー無し」
  // なので、effect内で同期的にsetStateする必要がない(lintのset-state-in-effect対策)。
  // リトライ時だけfetchLatest()(ボタン押下時も同じ)で表示を「読み込み中」に戻してから取り直す(フェーズ9-4)。
  function load() {
    getLatestAdvice()
      .then(setResult)
      .catch((err) => setError(toUserMessage(err)))
      .finally(() => setLoading(false))
  }

  function fetchLatest() {
    setLoading(true)
    setError(null)
    setSlow(false)
    load()
  }

  useEffect(() => {
    load()
  }, [])

  // 読み込み中だけタイマーを動かす。setSlow(true)はタイマーのコールバック(非同期)からのみ
  // 呼び、falseへの戻しはfetchLatest(イベント)で行うため、effect内で同期的にsetStateしない。
  useEffect(() => {
    if (!loading) return
    const id = setTimeout(() => setSlow(true), SLOW_THRESHOLD_MS)
    return () => clearTimeout(id)
  }, [loading])

  function applyFeedback(updated: Advice) {
    setResult((prev) => (prev?.advice?.id === updated.id ? { ...prev, advice: updated } : prev))
  }

  if (loading) {
    return slow && canGenerate ? (
      <div className="advice-view__waiting">
        <p>新しいAI提案を生成しています...</p>
        <p className="advice-view__waiting-note">
          直近の記録・目標・天候をもとにAIが練習メニューを考えています。30秒ほど、長いと1分以上かかる場合があります。
        </p>
      </div>
    ) : (
      <p>AI提案を確認中...</p>
    )
  }
  // 以前はエラー文だけを返しており、「最新のAI提案を確認する」ボタンごと消えて
  // ページを再読み込みしないと復帰できなかった(9-4)。AI生成の失敗は一時的な
  // ことが多い(外部APIの混雑等)ため、その場で再試行できるようにする。
  if (error) return <ErrorRetry message={error} onRetry={fetchLatest} />
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
      {/* ボタンを押しても内容が変わらない理由を伝える(9-4)。提案は前回生成から24時間は
          再生成せずキャッシュを返す設計(docs/adr/004、service/advice.goのadviceFreshWindow)。 */}
      {result.status === 'ready' && (
        <p className="advice-view__cache-note">
          AI提案は、前回の生成から24時間が経つまでは新しく生成されず、同じ提案が表示されます。
        </p>
      )}
    </div>
  )
}
