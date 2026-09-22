import { useEffect, useState } from 'react'
import type { Session } from '@supabase/supabase-js'
import { supabase } from '../lib/supabase'

// ログイン状態(Supabaseセッション)をアプリ全体で参照するためのフック。
// Supabase SDKがブラウザのlocalStorageにセッションを保持・自動リフレッシュしてくれるので、
// ここでは「今のセッションを取得する」「変化を購読する」だけでよい。
export function useSession() {
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)
  // パスワード再設定メールのリンクから戻ってきた状態(docs/adr/010参照)。
  // trueの間はsessionがあってもDashboardではなくResetPasswordFormを表示する。
  const [isPasswordRecovery, setIsPasswordRecovery] = useState(false)

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session)
      setLoading(false)
    })

    const { data: subscription } = supabase.auth.onAuthStateChange((event, newSession) => {
      if (event === 'PASSWORD_RECOVERY') {
        setIsPasswordRecovery(true)
      }
      setSession(newSession)
    })

    return () => subscription.subscription.unsubscribe()
  }, [])

  return { session, loading, isPasswordRecovery, clearPasswordRecovery: () => setIsPasswordRecovery(false) }
}
