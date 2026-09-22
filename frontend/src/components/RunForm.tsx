import { useState } from 'react'
import { createRun, updateRun, type Run, type RunInput } from '../lib/api'

type Props = {
  // 指定時は編集モード(PUT)、未指定時は新規作成モード(POST)。
  editingRun?: Run
  onSaved: (run: Run) => void
  onCancel?: () => void
}

// 新規作成・編集の両方に使うフォーム。
// distance_km/duration_secはBE側で自動計算するpace_sec_per_kmの元になる値なので、
// フォームでは距離・時間(分秒)のみ入力させ、ペースの入力欄は持たない。
export function RunForm({ editingRun, onSaved, onCancel }: Props) {
  const [distanceKm, setDistanceKm] = useState(editingRun ? String(editingRun.distance_km) : '')
  const [hours, setHours] = useState(editingRun ? String(Math.floor(editingRun.duration_sec / 3600)) : '')
  const [minutes, setMinutes] = useState(
    editingRun ? String(Math.floor((editingRun.duration_sec % 3600) / 60)) : '',
  )
  const [seconds, setSeconds] = useState(editingRun ? String(editingRun.duration_sec % 60) : '')
  const [runDate, setRunDate] = useState(editingRun?.run_date ?? '')
  const [rpe, setRpe] = useState(editingRun?.rpe != null ? String(editingRun.rpe) : '')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const durationSec = (Number(hours) || 0) * 3600 + (Number(minutes) || 0) * 60 + (Number(seconds) || 0)
    const input: RunInput = {
      distance_km: Number(distanceKm),
      duration_sec: durationSec,
      run_date: runDate,
      rpe: rpe === '' ? null : Number(rpe),
    }

    setSaving(true)
    try {
      const saved = editingRun ? await updateRun(editingRun.id, input) : await createRun(input)
      onSaved(saved)
      if (!editingRun) {
        setDistanceKm('')
        setHours('')
        setMinutes('')
        setSeconds('')
        setRunDate('')
        setRpe('')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="run-form" onSubmit={handleSubmit}>
      <label>
        距離(km)
        <input
          type="number"
          step="0.01"
          min="0.01"
          value={distanceKm}
          onChange={(e) => setDistanceKm(e.target.value)}
          required
        />
      </label>
      <label>
        時間
        <span className="run-form__duration">
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
        日付
        <input type="date" value={runDate} onChange={(e) => setRunDate(e.target.value)} required />
      </label>
      <label>
        RPE(体感的きつさ 1〜10・任意)
        <input
          type="number"
          min="1"
          max="10"
          value={rpe}
          onChange={(e) => setRpe(e.target.value)}
        />
      </label>

      {error && <p className="run-form__error">{error}</p>}

      <div className="run-form__actions">
        <button type="submit" disabled={saving}>
          {saving ? '保存中...' : editingRun ? '更新する' : '記録する'}
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
