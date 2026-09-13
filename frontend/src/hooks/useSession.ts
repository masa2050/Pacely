import { useEffect, useState } from 'react'
import type { Session } from '@supabase/supabase-js'
import { supabase } from '../lib/supabase'

// ログイン状態(Supabaseセッション)をアプリ全体で参照するためのフック。
// Supabase SDKがブラウザのlocalStorageにセッションを保持・自動リフレッシュしてくれるので、
// ここでは「今のセッションを取得する」「変化を購読する」だけでよい。
export function useSession() {
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session)
      setLoading(false)
    })

    const { data: subscription } = supabase.auth.onAuthStateChange((_event, newSession) => {
      setSession(newSession)
    })

    return () => subscription.subscription.unsubscribe()
  }, [])

  return { session, loading }
}
