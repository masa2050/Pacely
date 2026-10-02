import { useEffect, useState } from 'react'
import { listAdvices, type Advice } from '../lib/api'
import { AdviceCard, REST_MENU_TYPE, formatPace } from './AdviceCard'

type Props = {
  onBack: () => void
}

// BEの件数上限(service/advice.goのmaxAdviceHistory)と揃えた表示用の値。
// 返ってきた件数が上限に達している場合だけ「ここまでしか遡れない」旨を出す。
const HISTORY_LIMIT = 50

// 一覧の1行に出すメニュー要約。ADR-020の「日付 / メニュー(距離・ペース) / 評価」のうち
// メニュー列にあたる。8-2②でmenu_typeが追加されたため種別も併記する(docs/adr/020 影響)。
// 休養日はdistance_km/pace_sec_per_kmが0で返るため(docs/adr/021)、種別だけを出す。
function menuSummary(advice: Advice): string {
  const { menu_type, distance_km, pace_sec_per_km } = advice.next_menu
  if (menu_type === REST_MENU_TYPE) return REST_MENU_TYPE
  const body = `${distance_km}km / ${formatPace(pace_sec_per_km)}`
  // menu_typeが空なのは8-2②導入前に生成された古い提案。欄を埋めずに距離・ペースだけ出す。
  return menu_type ? `${menu_type} ${body}` : body
}

function feedbackMark(isHelpful: boolean | null): string {
  if (isHelpful === true) return '👍'
  if (isHelpful === false) return '👎'
  return '—'
}

// フェーズ9-5(docs/adr/020): AI提案履歴の専用画面。
//
// 以前はダッシュボードのAI提案カード内で、過去の提案をAdviceCardのまま縦に積んでいた。
// 実測でカード1件が約570px(記録は1行約34px)あり、件数が増えるほどダッシュボードが
// 下へ伸び続けていた。そこで各件を1行サマリーに圧縮し(クリックで全文を展開)、
// さらに履歴自体をダッシュボードから外して専用画面に移した。
// ルーティングライブラリは引き続き導入せず、App.tsxの画面切り替えに従う(docs/adr/015)。
export function AdviceHistory({ onBack }: Props) {
  const [advices, setAdvices] = useState<Advice[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // 展開は1件だけに絞る(アコーディオン)。全文が必要なのは特定の1件だけで、
  // 複数開けるようにすると結局画面が縦に伸びて元の問題に戻るため(docs/adr/020 理由2)。
  const [expandedId, setExpandedId] = useState<string | null>(null)

  function fetchHistory() {
    setLoading(true)
    setError(null)
    listAdvices()
      .then(setAdvices)
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchHistory()
  }, [])

  function applyFeedback(updated: Advice) {
    setAdvices((prev) => prev?.map((a) => (a.id === updated.id ? updated : a)) ?? prev)
  }

  return (
    <div className="advice-history">
      <button type="button" className="advice-history__back" onClick={onBack}>
        ← ダッシュボードに戻る
      </button>

      <h1>AI提案の履歴</h1>
      <p className="advice-history__lead">行をクリックすると、その日の提案の全文を表示します。</p>

      {loading && <p>読み込み中...</p>}

      {/* エラー時にリトライ手段ごと消えないよう、再読み込みボタンを添える。
          ページを再読み込みしないと復帰できない作りは9-4で全体的に見直す予定だが、
          新規画面で同じ問題を作らないためここでは最初から入れておく。 */}
      {error && (
        <div className="advice-history__error">
          <p className="dashboard__error">{error}</p>
          <button type="button" onClick={fetchHistory}>
            再読み込み
          </button>
        </div>
      )}

      {advices && advices.length === 0 && !error && <p>過去の提案はまだありません。</p>}

      {advices && advices.length > 0 && (
        <>
          {/* テーブルではなくul + 全幅buttonにしている。行全体をクリックで展開したいが、
              tr自体はボタンにできず、キーボード操作(Enter/Space)やaria-expandedも
              自前で補う必要があるため。列の揃えはCSS Gridで行う(App.css)。 */}
          <ul className="advice-history__list">
            {advices.map((advice) => {
              const expanded = expandedId === advice.id
              return (
                <li key={advice.id} className="advice-history__item">
                  <button
                    type="button"
                    className={`advice-history__row${expanded ? ' advice-history__row--expanded' : ''}`}
                    onClick={() => setExpandedId(expanded ? null : advice.id)}
                    aria-expanded={expanded}
                  >
                    <span className="advice-history__date">
                      {new Date(advice.generated_at).toLocaleDateString('ja-JP')}
                    </span>
                    <span className="advice-history__menu">{menuSummary(advice)}</span>
                    <span
                      className="advice-history__feedback"
                      aria-label={
                        advice.is_helpful === true
                          ? '役に立った'
                          : advice.is_helpful === false
                            ? '役に立たなかった'
                            : '未評価'
                      }
                    >
                      {feedbackMark(advice.is_helpful)}
                    </span>
                  </button>

                  {expanded && (
                    <div className="advice-history__detail">
                      <AdviceCard advice={advice} onFeedbackSaved={applyFeedback} />
                    </div>
                  )}
                </li>
              )
            })}
          </ul>

          {advices.length >= HISTORY_LIMIT && (
            <p className="advice-history__limit-note">直近{HISTORY_LIMIT}件までを表示しています。</p>
          )}
        </>
      )}
    </div>
  )
}
