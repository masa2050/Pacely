import { useState } from 'react'
import { createGoal, updateGoal, type Goal, type GoalInput } from '../lib/api'

type Props = {
  // 指定時は編集モード(PUT、目標タイムなどの修正)、未指定時は新規作成モード(POST)。
  editingGoal?: Goal
  onSaved: (goal: Goal) => void
  onCancel?: () => void
}

export const GOAL_TYPES = [
  { value: 'full_marathon', label: 'フルマラソン' },
  { value: 'half_marathon', label: 'ハーフマラソン' },
  { value: '10km', label: '10km' },
  { value: '5km', label: '5km' },
]

// 新規作成・編集の両方に使うフォーム。
// 新規作成時、既存のactive目標があればBE側で自動的にabandoned(断念)に更新される
// (docs/api.md 2.3、docs/adr/009)。
export function GoalForm({ editingGoal, onSaved, onCancel }: Props) {
  const [goalType, setGoalType] = useState(editingGoal?.goal_type ?? GOAL_TYPES[0].value)
  const [hours, setHours] = useState(editingGoal ? String(Math.floor(editingGoal.target_time_sec / 3600)) : '')
  const [minutes, setMinutes] = useState(
    editingGoal ? String(Math.floor((editingGoal.target_time_sec % 3600) / 60)) : '',
  )
  const [seconds, setSeconds] = useState(editingGoal ? String(editingGoal.target_time_sec % 60) : '')
  const [targetDate, setTargetDate] = useState(editingGoal?.target_date ?? '')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const targetTimeSec = (Number(hours) || 0) * 3600 + (Number(minutes) || 0) * 60 + (Number(seconds) || 0)
    const input: GoalInput = {
      goal_type: goalType,
      target_time_sec: targetTimeSec,
      target_date: targetDate,
    }

    setSaving(true)
    try {
      const saved = editingGoal ? await updateGoal(editingGoal.id, input) : await createGoal(input)
      onSaved(saved)
      if (!editingGoal) {
        setHours('')
        setMinutes('')
        setSeconds('')
        setTargetDate('')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="goal-form" onSubmit={handleSubmit}>
      <label>
        種目
        <select value={goalType} onChange={(e) => setGoalType(e.target.value)}>
          {GOAL_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        目標タイム
        <span className="goal-form__duration">
          <input type="number" min="0" placeholder="時" value={hours} onChange={(e) => setHours(e.target.value)} />
          時間
          <input
            type="number"
            min="0"
            max="59"
            placeholder="分"
            value={minutes}
            onChange={(e) => setMinutes(e.target.value)}
          />
          分
          <input
            type="number"
            min="0"
            max="59"
            placeholder="秒"
            value={seconds}
            onChange={(e) => setSeconds(e.target.value)}
          />
          秒
        </span>
      </label>
      <label>
        目標達成予定日
        <input type="date" value={targetDate} onChange={(e) => setTargetDate(e.target.value)} required />
      </label>

      {error && <p className="goal-form__error">{error}</p>}

      <div className="goal-form__actions">
        <button type="submit" disabled={saving}>
          {saving ? '保存中...' : editingGoal ? '更新する' : '目標を設定する'}
        </button>
        {onCancel && (
          <button type="button" onClick={onCancel} disabled={saving}>
            キャンセル
          </button>
        )}
      </div>
    </form>
  )
}
