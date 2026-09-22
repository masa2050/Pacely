import { useState } from 'react'
import { updateGoalStatus, type Goal } from '../lib/api'
import { GoalForm, GOAL_TYPES } from './GoalForm'

type Props = {
  // 現在有効な目標(GET /goals/active)。無ければnull。
  activeGoal: Goal | null
  // 目標一覧(GET /goals、履歴含む)。
  goals: Goal[]
  onActiveChanged: (goal: Goal | null) => void
  onGoalsChanged: (goals: Goal[]) => void
}

const GOAL_TYPE_LABELS: Record<string, string> = Object.fromEntries(GOAL_TYPES.map((t) => [t.value, t.label]))

const STATUS_LABELS: Record<Goal['status'], string> = {
  active: '進行中',
  achieved: '達成',
  abandoned: '断念',
}

function formatTargetTime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

// 「現在の目標表示」と「目標履歴一覧」をまとめて担当するコンポーネント
// (docs/implementation-plan.md フェーズ3)。
export function GoalList({ activeGoal, goals, onActiveChanged, onGoalsChanged }: Props) {
  const [editingActive, setEditingActive] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const history = goals.filter((g) => g.id !== activeGoal?.id)

  async function handleStatusChange(status: 'achieved' | 'abandoned') {
    if (!activeGoal) return
    setError(null)
    try {
      const updated = await updateGoalStatus(activeGoal.id, status)
      onActiveChanged(null)
      onGoalsChanged(goals.map((g) => (g.id === updated.id ? updated : g)))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="goal-list">
      {error && <p className="goal-list__error">{error}</p>}

      <h3>現在の目標</h3>
      {activeGoal ? (
        editingActive ? (
          <GoalForm
            editingGoal={activeGoal}
            onSaved={(updated) => {
              onActiveChanged(updated)
              onGoalsChanged(goals.map((g) => (g.id === updated.id ? updated : g)))
              setEditingActive(false)
            }}
            onCancel={() => setEditingActive(false)}
          />
        ) : (
          <div className="goal-list__active">
            <p>
              {GOAL_TYPE_LABELS[activeGoal.goal_type] ?? activeGoal.goal_type} / 目標タイム{' '}
              {formatTargetTime(activeGoal.target_time_sec)} / 期限 {activeGoal.target_date}
            </p>
            <div className="goal-list__actions">
              <button type="button" onClick={() => setEditingActive(true)}>
                編集
              </button>
              <button type="button" onClick={() => handleStatusChange('achieved')}>
                達成にする
              </button>
              <button type="button" onClick={() => handleStatusChange('abandoned')}>
                断念する
              </button>
            </div>
          </div>
        )
      ) : (
        <p>現在有効な目標はありません。下のフォームから設定しましょう。</p>
      )}

      {history.length > 0 && (
        <>
          <h3>目標履歴</h3>
          <table>
            <thead>
              <tr>
                <th>種目</th>
                <th>目標タイム</th>
                <th>期限</th>
                <th>状態</th>
              </tr>
            </thead>
            <tbody>
              {history.map((g) => (
                <tr key={g.id}>
                  <td>{GOAL_TYPE_LABELS[g.goal_type] ?? g.goal_type}</td>
                  <td>{formatTargetTime(g.target_time_sec)}</td>
                  <td>{g.target_date}</td>
                  <td>{STATUS_LABELS[g.status]}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
