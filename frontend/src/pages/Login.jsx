import { useState } from 'react'
import { useAuth } from '../auth'

export default function Login() {
  const { login, user } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  if (user) window.location.hash = '#/'

  const submit = async (e) => {
    e.preventDefault()
    setErr('')
    setBusy(true)
    try {
      await login(username, password)
      window.location.hash = '#/'
    } catch (e2) {
      setErr(e2.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 380, margin: '80px auto', padding: 16 }}>
      <div className="card">
        <h1 style={{ marginTop: 0 }}>Kios</h1>
        <p className="muted">Operasional harian kios</p>
        <form onSubmit={submit}>
          <input
            placeholder="Username" value={username}
            onChange={(e) => setUsername(e.target.value)}
            style={{ width: '100%', marginBottom: 10 }}
            autoFocus
          />
          <input
            placeholder="Password" type="password" value={password}
            onChange={(e) => setPassword(e.target.value)}
            style={{ width: '100%', marginBottom: 14 }}
          />
          {err && <p className="danger-text">{err}</p>}
          <button className="btn-accent" style={{ width: '100%' }} disabled={busy}>
            {busy ? 'Memproses…' : 'Masuk'}
          </button>
        </form>
      </div>
    </div>
  )
}
