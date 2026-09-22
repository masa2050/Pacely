import { useEffect, useState } from 'react'
import { useSession } from './hooks/useSession'
import { AuthForm } from './components/AuthForm'
import { Dashboard } from './components/Dashboard'
import { Settings } from './components/Settings'
import { ResetPasswordForm } from './components/ResetPasswordForm'
import { PrivacyPolicy } from './components/PrivacyPolicy'
import './App.css'

function App() {
  const { session, loading, isPasswordRecovery, clearPasswordRecovery } = useSession()
  // プライバシーポリシーは専用ルーターを導入せず、ログイン状態に関わらず
  // 表示できる簡易な画面切り替えとして扱う(過剰設計を避ける方針)。
  const [showPrivacyPolicy, setShowPrivacyPolicy] = useState(false)
  // 設定画面も同様に、react-router-domを新規導入せず既存の画面切り替えパターンを踏襲する
  // (フェーズ7-2、docs/adr/015参照)。
  const [showSettings, setShowSettings] = useState(false)

  // 設定画面からの退会(Settings.tsx)はsupabase.auth.signOut()でsessionをnullにするが、
  // showSettingsはApp.tsxのローカルstateなので自動では戻らない。session側でリセットし、
  // ログアウト・退会のどちらでも確実にログイン画面(AuthForm)へ戻るようにする。
  useEffect(() => {
    if (!session) {
      setShowSettings(false)
    }
  }, [session])

  if (loading) {
    return null
  }

  if (showPrivacyPolicy) {
    return (
      <div id="center">
        <PrivacyPolicy onBack={() => setShowPrivacyPolicy(false)} />
      </div>
    )
  }

  // パスワード再設定リンクからの遷移時は、セッションの有無に関わらず
  // ResetPasswordFormを優先して表示する(docs/adr/010参照)。
  if (isPasswordRecovery) {
    return (
      <div id="center">
        <ResetPasswordForm onDone={clearPasswordRecovery} />
      </div>
    )
  }

  if (showSettings && session) {
    return (
      <div id="center">
        <Settings onBack={() => setShowSettings(false)} />
      </div>
    )
  }

  return (
    <div id="center">
      {session ? (
        <Dashboard session={session} onSettingsClick={() => setShowSettings(true)} />
      ) : (
        <AuthForm />
      )}
      <button type="button" className="app__privacy-link" onClick={() => setShowPrivacyPolicy(true)}>
        プライバシーポリシー・利用規約
      </button>
    </div>
  )
}

export default App
