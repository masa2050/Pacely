import { useSession } from './hooks/useSession'
import { AuthForm } from './components/AuthForm'
import { Dashboard } from './components/Dashboard'
import './App.css'

function App() {
  const { session, loading } = useSession()

  if (loading) {
    return null
  }

  return (
    <div id="center">
      {session ? <Dashboard session={session} /> : <AuthForm />}
    </div>
  )
}

export default App
