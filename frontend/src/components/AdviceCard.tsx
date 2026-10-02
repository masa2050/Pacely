import { useState } from 'react'
import { submitAdviceFeedback, type Advice } from '../lib/api'

// フェーズ9-5(docs/adr/020)で、AI提案1件分の表示をAdviceViewから切り出した。
// ダッシュボードの「本日の提案」(AdviceView)と提案履歴画面(AdviceHistory)の
// 両方が同じ見た目でAI提案を表示するため、共通部品として独立させている。

export function formatPace(paceSecPerKm: number): string {
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

// 8-2②(docs/adr/021)でmenu_typeが「休養」の場合、distance_km/pace_sec_per_kmは
// プロンプト指示上0で返る(BE ai_client.goのbuildPrompt参照)。0km・0:00/kmと
// 表示すると練習メニューに見えて紛らわしいため、休養日は種別とnoteだけを表示する。
// 履歴画面の1行サマリーでも同じ判定が要るためexportしている。
export const REST_MENU_TYPE = '休養'

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

export function AdviceCard({
  advice,
  onFeedbackSaved,
}: {
  advice: Advice
  onFeedbackSaved: (updated: Advice) => void
}) {
  const { menu_type, distance_km, pace_sec_per_km, note, segments } = advice.next_menu
  const isRest = menu_type === REST_MENU_TYPE
  return (
    <div className="advice-view__card">
      <p className="advice-view__text">{advice.advice_text}</p>

      {/* 「次回」だと誤解を招く(このメニューはその日の24時間キャッシュ対象、docs/adr/004)ため
          「本日の練習メニュー」と表記する。距離・ペース・合計時間はdt/ddの縦積みではなく
          横並びの統計表示にして、狭い画面でも縦に間延びしないようにする。 */}
      <div className="advice-view__menu">
        <p className="advice-view__menu-label">本日の練習メニュー</p>
        <div className="advice-view__stats">
          {/* menu_typeが空なのは8-2②導入前に生成された提案(履歴に残っている)。
              「不明」と表示するとエラーのように見えるが、実際には当時のスキーマに
              種別が無かっただけなので、欄自体を出さず②導入前と同じ表示に戻す。 */}
          {menu_type && (
            <div className="advice-view__stat">
              <span className="advice-view__stat-label">種別</span>
              <span className="advice-view__stat-value">{menu_type}</span>
            </div>
          )}
          {!isRest && (
            <>
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
            </>
          )}
        </div>
        {/* segmentsは現時点でビルドアップ走のみ入る(docs/adr/022)。distance_km/
            pace_sec_per_kmという合計・代表値だけでは表現できない、区間ごとの
            ペース変化の内訳を補足として表示する。無ければ何も出さず、従来通り
            上のstatsだけで完結する。 */}
        {segments && segments.length > 0 && (
          <ol className="advice-view__segments">
            {segments.map((seg, i) => (
              <li key={i}>
                {seg.reps > 1 ? `${seg.reps}本 × ` : ''}
                {seg.distance_km}km / {formatPace(seg.pace_sec_per_km)}
                {seg.rest_sec > 0 && `(レスト${seg.rest_sec}秒)`}
              </li>
            ))}
          </ol>
        )}
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
