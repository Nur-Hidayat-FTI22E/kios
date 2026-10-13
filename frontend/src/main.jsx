import React, { useState, useEffect } from 'react'
import ReactDOM from 'react-dom/client'
import { HashRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './auth'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Kasir from './pages/Kasir'
import Produk from './pages/Produk'
import Stok from './pages/Stok'
import Pembelian from './pages/Pembelian'
import Utang from './pages/Utang'
import Pengeluaran from './pages/Pengeluaran'
import TutupKasir from './pages/TutupKasir'
import Laporan from './pages/Laporan'
import Pengguna from './pages/Pengguna'
import './styles/tokens.css'
import './styles/app.css'

const qc = new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } } })

function useTema() {
  const [tema, setTema] = useState(() => (localStorage.getItem('tema') === 'gelap' ? 'gelap' : 'terang'))
  useEffect(() => {
    document.documentElement.dataset.theme = tema === 'gelap' ? 'dark' : ''
    localStorage.setItem('tema', tema)
  }, [tema])
  return [tema, () => setTema((t) => (t === 'gelap' ? 'terang' : 'gelap'))]
}

const SEMUA_LINK = [
  ['/', 'Dasbor', '🏠'],
  ['/kasir', 'Kasir', '🛒'],
  ['/produk', 'Produk', '📦'],
  ['/stok', 'Stok', '🗂️'],
  ['/pembelian', 'Belanja', '🧾'],
  ['/utang', 'Utang', '💳'],
  ['/pengeluaran', 'Biaya', '💸'],
  ['/tutup-kasir', 'Tutup Kasir', '🧮'],
  ['/laporan', 'Laporan', '📈'],
  ['/pengguna', 'Pengguna', '👤'],
]

function Shell({ children }) {
  const { user, logout } = useAuth()
  const isPemilik = user?.peran === 'pemilik'
  const [tema, gantiTema] = useTema()
  const links = SEMUA_LINK.filter(([to]) => to !== '/laporan' && to !== '/pengguna' || isPemilik)
  return (
    <div className="app-shell">
      <header className="topbar card">
        <span className="brand">Kios</span>
        <span className="muted user-chip">{user?.nama} · {user?.peran}</span>
        <span style={{ flex: 1 }} />
        <button onClick={gantiTema} title="Ganti tema terang/gelap">
          {tema === 'gelap' ? '☀️ Terang' : '🌙 Gelap'}
        </button>
        <button onClick={logout}>Keluar</button>
      </header>
      {/* Desktop: satu baris pendek di bawah topbar; HP: tab bawah (CSS) */}
      <nav className="desknav">
        {links.map(([to, label, ico]) => (
          <NavLink key={to} to={to} className={({ isActive }) => (isActive ? 'active' : '')}>{ico} {label}</NavLink>
        ))}
      </nav>
      <main className="content">{children}</main>
      <nav className="tabbar">
        {links.map(([to, label, ico]) => (
          <NavLink key={to} to={to} className={({ isActive }) => (isActive ? 'active' : '')}>
            <span className="ico">{ico}</span>
            <span className="lbl">{label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  )
}

function Protected({ pemilikOnly = false, children }) {
  const { user, loading } = useAuth()
  if (loading) return <p className="muted">Memuat…</p>
  if (!user) return <Navigate to="/login" replace />
  if (pemilikOnly && user.peran !== 'pemilik') {
    return <p className="danger-text">Halaman ini hanya untuk pemilik.</p>
  }
  return children
}

function App() {
  return (
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <HashRouter>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="*" element={
              <Shell>
                <Routes>
                  <Route path="/" element={<Protected><Dashboard /></Protected>} />
                  <Route path="/kasir" element={<Protected><Kasir /></Protected>} />
                  <Route path="/produk" element={<Protected><Produk /></Protected>} />
                  <Route path="/stok" element={<Protected><Stok /></Protected>} />
                  <Route path="/pembelian" element={<Protected><Pembelian /></Protected>} />
                  <Route path="/utang" element={<Protected><Utang /></Protected>} />
                  <Route path="/pengeluaran" element={<Protected><Pengeluaran /></Protected>} />
                  <Route path="/tutup-kasir" element={<Protected><TutupKasir /></Protected>} />
                  <Route path="/laporan" element={<Protected pemilikOnly><Laporan /></Protected>} />
                  <Route path="/pengguna" element={<Protected pemilikOnly><Pengguna /></Protected>} />
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </Shell>
            } />
          </Routes>
        </HashRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />)
