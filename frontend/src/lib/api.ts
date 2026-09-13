import { supabase } from './supabase'

// docs/architecture.md: フロントエンドはGoバックエンドとのみ通信する。
// Claude API・天候APIへはフロントから直接アクセスしない。
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export type Me = {
  id: string
  region: string | null
  created_at: string
}

type ApiErrorBody = { message?: string }

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
    throw new Error(body.message ?? `APIエラー(status ${res.status})`)
  }
  return res.json() as Promise<T>
}

export function getMe(): Promise<Me> {
  return authorizedFetch<Me>('/users/me')
}

export function updateMyRegion(region: string): Promise<Me> {
  return authorizedFetch<Me>('/users/me', {
    method: 'PUT',
    body: JSON.stringify({ region }),
  })
}
