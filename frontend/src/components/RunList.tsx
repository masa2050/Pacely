import { useState } from 'react'
import { deleteRun, type Run } from '../lib/api'
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

  async function handleDelete(id: string) {
    setError(null)
    try {
      await deleteRun(id)
      onChanged(runs.filter((r) => r.id !== id))
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
                      onChanged(runs.map((r) => (r.id === updated.id ? updated : r)))
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
                <td>
                  <button type="button" onClick={() => setEditingId(run.id)}>
                    編集
                  </button>
                  <button type="button" onClick={() => handleDelete(run.id)}>
                    削除
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
