import { supabase } from './supabase'

// docs/architecture.md: フロントエンドはGoバックエンドとのみ通信する。
// Claude API・天候APIへはフロントから直接アクセスしない。
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export type Me = {
  id: string
  region: string | null
  username: string | null
  created_at: string
}

type ApiErrorBody = { message?: string }

// ステータスコードを保持するエラー。GET /goals/active の404(有効な目標なし)のように、
// エラーの種類によって呼び出し側で分岐したい場合に使う。
export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// バックエンドの全エンドポイントはJWT検証必須(docs/api.md)なので、
// Supabaseの現在のセッションからaccess_tokenを取り出してAuthorizationヘッダーに付与する。
async function authorizedFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const { data } = await supabase.auth.getSession()
  const token = data.session?.access_token
  if (!token) {
    throw new Error('ログインしていません')
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  })

  if (!res.ok) {
    const body: ApiErrorBody = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.message ?? `APIエラー(status ${res.status})`)
  }
  // DELETE(204 No Content)はボディが無いため、res.json()を呼ぶとパースエラーになる。
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export function getMe(): Promise<Me> {
  return authorizedFetch<Me>('/users/me')
}

// region・usernameはどちらか一方のみの更新にも対応する(バックエンド側でPUTボディに
// 含まれなかったフィールドは変更しない、docs/api.md 2.6)。
export function updateMyProfile(input: { region?: string; username?: string }): Promise<Me> {
  return authorizedFetch<Me>('/users/me', {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

// 退会。Pacely独自データとSupabase Authアカウントを両方削除する(docs/adr/014)。
export function deleteAccount(): Promise<void> {
  return authorizedFetch<void>('/users/me', { method: 'DELETE' })
}

// ログイン中ユーザーのパスワード変更(フェーズ7-4)。呼び出し前に呼び出し側で
// supabase.auth.signInWithPasswordによる現在のパスワードの再認証を済ませておく前提
// (docs/adr/016)。
export function changePassword(newPassword: string): Promise<void> {
  return authorizedFetch<void>('/users/me/password', {
    method: 'PUT',
    body: JSON.stringify({ new_password: newPassword }),
  })
}

export type Run = {
  id: string
  user_id: string
  distance_km: number
  duration_sec: number
  pace_sec_per_km: number
  rpe: number | null
  run_date: string
  created_at: string
}

export type RunInput = {
  distance_km: number
  duration_sec: number
  run_date: string
  rpe: number | null
}

export function listRuns(): Promise<Run[]> {
  return authorizedFetch<Run[]>('/runs')
}

export function createRun(input: RunInput): Promise<Run> {
  return authorizedFetch<Run>('/runs', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateRun(id: string, input: RunInput): Promise<Run> {
  return authorizedFetch<Run>(`/runs/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteRun(id: string): Promise<void> {
  return authorizedFetch<void>(`/runs/${id}`, {
    method: 'DELETE',
  })
}

export type GoalStatus = 'active' | 'achieved' | 'abandoned'

export type Goal = {
  id: string
  user_id: string
  goal_type: string
  target_time_sec: number
  target_date: string
  status: GoalStatus
  created_at: string
}

export type GoalInput = {
  goal_type: string
  target_time_sec: number
  target_date: string
}

export function listGoals(): Promise<Goal[]> {
  return authorizedFetch<Goal[]>('/goals')
}

// 有効な目標が無い場合、バックエンドは404を返す(docs/api.md)。
// UI側では「未設定」として扱いたいので、404だけはnullに変換する。
export async function getActiveGoal(): Promise<Goal | null> {
  try {
    return await authorizedFetch<Goal>('/goals/active')
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null
    }
    throw err
  }
}

export function createGoal(input: GoalInput): Promise<Goal> {
  return authorizedFetch<Goal>('/goals', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateGoal(id: string, input: GoalInput): Promise<Goal> {
  return authorizedFetch<Goal>(`/goals/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function updateGoalStatus(id: string, status: 'achieved' | 'abandoned'): Promise<Goal> {
  return authorizedFetch<Goal>(`/goals/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export type Progress = {
  goal: Goal
  target_pace_sec_per_km: number
  // 記録が1件も無い場合はバックエンド側でnullになる(docs/api.md 2.4)。
  recent_average_pace_sec_per_km: number | null
  recent_runs_count: number
  pace_diff_sec_per_km: number | null
}

// 有効な目標が無い場合、バックエンドは404を返す。getActiveGoalと同様、
// UI側では「未設定」として扱いたいのでnullに変換する。
export async function getActiveGoalProgress(): Promise<Progress | null> {
  try {
    return await authorizedFetch<Progress>('/goals/active/progress')
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null
    }
    throw err
  }
}

export type NextMenu = {
  distance_km: number
  pace_sec_per_km: number
  note: string
}

export type WeatherContext = {
  has_weather: boolean
  description?: string
  temp_c?: number
}

export type Advice = {
  id: string
  user_id: string
  goal_id: string | null
  advice_text: string
  next_menu: NextMenu
  weather_context: WeatherContext
  generated_at: string
}

// GET /advices/latest はAI呼び出しをスキップした場合(記録不足・目標未設定)も
// 200で返ってくる(docs/api.md 3.)。statusでフロント側の表示を分岐する。
export type AdviceStatus = 'ready' | 'needs_more_runs' | 'needs_active_goal'

export type LatestAdviceResult = {
  status: AdviceStatus
  advice?: Advice
  runs_count: number
  min_runs_required: number
}

// このリクエストは意図的にGETに副作用を持つ(生成判定→必要ならAI呼び出し・DB保存、
// docs/adr/004)。呼び出し側は「ボタンを押すたび生成される」わけではない点に注意。
export function getLatestAdvice(): Promise<LatestAdviceResult> {
  return authorizedFetch<LatestAdviceResult>('/advices/latest')
}

export function listAdvices(): Promise<Advice[]> {
  return authorizedFetch<Advice[]>('/advices')
}
