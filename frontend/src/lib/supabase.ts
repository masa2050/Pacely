import { createClient } from '@supabase/supabase-js'

const supabaseUrl = import.meta.env.VITE_SUPABASE_URL
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY

if (!supabaseUrl || !supabaseAnonKey) {
  // 起動直後に気づけるよう、早めに落とす。
  throw new Error(
    'VITE_SUPABASE_URL / VITE_SUPABASE_ANON_KEY が未設定です。frontend/.env を確認してください。',
  )
}

// アプリ全体で1つのクライアントを共有する(Supabaseのセッション管理はこのインスタンスが担う)。
export const supabase = createClient(supabaseUrl, supabaseAnonKey)
