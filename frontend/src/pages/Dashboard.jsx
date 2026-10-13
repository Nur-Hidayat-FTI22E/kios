import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, rupiah, tanggalRingkas } from '../api/client'

export default function Dashboard() {
  const { data, isLoading } = useQuery({ queryKey: ['dashboard'], queryFn: () => api('/reports/dashboard') })

  if (isLoading || !data) return <p className="muted">Memuat dasbor…</p>

  const isPemilik = (() => {
    try { return JSON.parse(atob(localStorage.getItem('token').split('.')[1])).peran === 'pemilik' } catch { return false }
  })()

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
        <h1 style={{ margin: 0 }}>Dasbor</h1>
        <Link to="/kasir" className="btn-accent" style={{ textDecoration: 'none' }}>
          <button className="btn-accent">Buka Kasir</button>
        </Link>
      </div>
      <div className="stat-cards" style={{ marginBottom: 16 }}>
        <div className="card"><h3>Omzet hari ini</h3><div className="val">{rupiah(data.omzet_hari_ini)}</div></div>
        <div className="card"><h3>Jumlah transaksi</h3><div className="val">{data.jumlah_transaksi}</div></div>
        {isPemilik && (
          <div className="card"><h3>Laba kotor hari ini</h3><div className="val">{rupiah(data.laba_kotor_hari_ini)}</div></div>
        )}
        <div className="card"><h3>Utang pelanggan</h3><div className="val">{rupiah(data.total_utang_pelanggan)}</div></div>
        <div className="card"><h3>Utang supplier</h3><div className="val">{rupiah(data.total_utang_supplier)}</div></div>
      </div>
      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Stok menipis</h3>
          {(data.stok_menipis || []).length === 0 && <p className="muted">Tidak ada.</p>}
          <ul>
            {[...(data.stok_menipis || [])].map((p) => (
              <li key={p.id}>
                {p.nama} — <span className="warning-text">sisa {p.stok} (min {p.stok_minimum})</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Utang jatuh tempo / terlama</h3>
          {(data.utang_jatuh_tempo || []).length === 0 && <p className="muted">Tidak ada.</p>}
          <ul>
            {[...(data.utang_jatuh_tempo || [])].map((d) => (
              <li key={d.id}>
                {d.pihak || '(tanpa nama)'} — {rupiah(d.sisa)}
                {d.jatuh_tempo && <> · tempo {tanggalRingkas(d.jatuh_tempo)}</>}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </>
  )
}
