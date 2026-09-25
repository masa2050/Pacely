import { useState } from 'react'
import { deleteRun, sortRuns, type Run } from '../lib/api'
import { RunForm } from './RunForm'

type Props = {
  runs: Run[]
  onChanged: (runs: Run[]) => void
}

function formatPace(paceSecPerKm: number): string {
  const min = Math.floor(paceSecPerKm / 60)
  const sec = Math.round(paceSecPerKm % 60)
  return `${min}:${String(sec).padStart(2, '0')}/km`
}

// 記録一覧の表示・編集・削除をまとめて担当するコンポーネント。
// 編集中は行をRunFormに差し替える(一覧画面の中でインライン編集する形)。
export function RunList({ runs, onChanged }: Props) {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  // 退会(Settings.tsx)と同じくwindow.confirmによる簡易確認。取り消せない削除の
  // 前に誤タップを防ぐのが目的で、専用の確認モーダルを作るほどの複雑さは不要と判断した。
  async function handleDelete(run: Run) {
    if (!window.confirm(`${run.run_date}の記録(${run.distance_km}km)を削除します。元に戻せません。よろしいですか?`)) {
      return
    }
    setError(null)
    try {
      await deleteRun(run.id)
      onChanged(runs.filter((r) => r.id !== run.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  if (runs.length === 0) {
    return <p>まだ記録がありません。上のフォームから記録してみましょう。</p>
  }

  return (
    <div className="run-list">
      {error && <p className="run-list__error">{error}</p>}
      <table>
        {/* table-layout:fixed(App.css)で列幅を固定するため、割合はcolgroupで指定する。
            これによりRPE・編集/削除ボタンまで含めてカード幅に収まり、横スクロールが
            不要になる(縦スクロールはrun-list自体に残したまま)。 */}
        <colgroup>
          <col className="run-list__col-date" />
          <col className="run-list__col-distance" />
          <col className="run-list__col-pace" />
          <col className="run-list__col-rpe" />
          <col className="run-list__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>日付</th>
            <th>距離</th>
            <th>ペース</th>
            <th>RPE</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run) =>
            editingId === run.id ? (
              <tr key={run.id}>
                <td colSpan={5}>
                  <RunForm
                    editingRun={run}
                    onSaved={(updated) => {
                      onChanged(sortRuns(runs.map((r) => (r.id === updated.id ? updated : r))))
                      setEditingId(null)
                    }}
                    onCancel={() => setEditingId(null)}
                  />
                </td>
              </tr>
            ) : (
              <tr key={run.id}>
                <td>{run.run_date}</td>
                <td>{run.distance_km}km</td>
                <td>{formatPace(run.pace_sec_per_km)}</td>
                <td>{run.rpe ?? '-'}</td>
                <td className="run-list__actions">
                  {/* 「編集」「削除」のテキストボタンは横幅を取り、狭いカード幅
                      (PC3カラム表示時など)でテーブルが横スクロール必須になって
                      いたため、アイコンボタンに変更して省スペース化した。 */}
                  <button
                    type="button"
                    className="run-list__icon-button"
                    onClick={() => setEditingId(run.id)}
                    aria-label="編集"
                    title="編集"
                  >
                    ✎
                  </button>
                  <button
                    type="button"
                    className="run-list__icon-button"
                    onClick={() => handleDelete(run)}
                    aria-label="削除"
                    title="削除"
                  >
                    🗑
                  </button>
                </td>
              </tr>
            ),
          )}
        </tbody>
      </table>
    </div>
  )
}
