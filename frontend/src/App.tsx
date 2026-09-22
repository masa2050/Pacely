import { useState } from 'react'
import { useSession } from './hooks/useSession'
import { AuthForm } from './components/AuthForm'
import { Dashboard } from './components/Dashboard'
import { ResetPasswordForm } from './components/ResetPasswordForm'
import { PrivacyPolicy } from './components/PrivacyPolicy'
import './App.css'

function App() {
  const { session, loading, isPasswordRecovery, clearPasswordRecovery } = useSession()
  // プライバシーポリシーは専用ルーターを導入せず、ログイン状態に関わらず
  // 表示できる簡易な画面切り替えとして扱う(過剰設計を避ける方針)。
  const [showPrivacyPolicy, setShowPrivacyPolicy] = useState(false)

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

  return (
    <div id="center">
      {session ? <Dashboard session={session} /> : <AuthForm />}
      <button type="button" className="app__privacy-link" onClick={() => setShowPrivacyPolicy(true)}>
        プライバシーポリシー・利用規約
      </button>
    </div>
  )
}

export default App
