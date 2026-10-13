import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, Legend } from 'recharts'
import { api, rupiah, getToken } from '../api/client'

function periodeDefault(what) {
  const hariini = new Date()
  const iso = (d) => { const x = new Date(d); x.setMinutes(x.getMinutes() - x.getTimezoneOffset()); return x.toISOString().slice(0, 10) }
  if (what === '7_hari') return [iso(Date.now() - 6 * 86400000), iso(hariini)]
  if (what === 'bulan_lalu') {
    const first = new Date(hariini.getFullYear(), hariini.getMonth() - 1, 1)
    const last = new Date(hariini.getFullYear(), hariini.getMonth(), 0)
    return [iso(first), iso(last)]
  }
  return [iso(new Date(hariini.getFullYear(), hariini.getMonth(), 1)), iso(hariini)]
}

export default function Laporan() {
  const [preset, setPreset] = useState('bulan_ini')
  const [dari, setDari] = useState(periodeDefault('bulan_ini')[0])
  const [sampai, setSampai] = useState(periodeDefault('bulan_ini')[1])
  const [eksporSibuk, setEksporSibuk] = useState(false)
  const [pesan, setPesan] = useState(null)

  const pilih = (p) => {
    setPreset(p)
    if (p === 'kustom') return
    const [d, s] = periodeDefault(p)
    setDari(d); setSampai(s)
  }

  const params = `dari=${dari}&sampai=${sampai}`
  const { data: summary } = useQuery({ queryKey: ['lap-summary', dari, sampai], queryFn: () => api('/reports/summary?' + params) })
  const { data: trend } = useQuery({ queryKey: ['lap-trend', dari, sampai], queryFn: () => api('/reports/daily-trend?' + params) })
  const { data: top } = useQuery({ queryKey: ['lap-top', dari, sampai], queryFn: () => api('/reports/top-products?' + params) })

  // Ekspor Excel: unduh via fetch dengan token, bukan window.open
  // (endpoint /reports/export wajib header Authorization — window.open tidak mengirimnya)
  const ekspor = async () => {
    setPesan(null)
    setEksporSibuk(true)
    try {
      const resp = await fetch(`/api/v1/reports/export?${params}`, {
        headers: { Authorization: 'Bearer ' + getToken() },
      })
      if (!resp.ok) throw new Error('Ekspor gagal (' + resp.status + ')')
      const blob = await resp.blob()
      const m = (resp.headers.get('Content-Disposition') || '').match(/"([^"]+)"/)
      const nama = m ? m[1] : `laporan-kios_${dari}_${sampai}.xlsx`
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = nama
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      setPesan({ ok: true, text: 'Laporan Excel berhasil diunduh: ' + nama })
    } catch (e) {
      setPesan({ ok: false, text: e.message })
    } finally {
      setEksporSibuk(false)
    }
  }

  return (
    <>
      <h1>Laporan</h1>
      <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap', alignItems: 'center' }}>
        {[['hari_ini', 'Hari ini'], ['7_hari', '7 hari'], ['bulan_ini', 'Bulan ini'], ['bulan_lalu', 'Bulan lalu'], ['kustom', 'Kustom']].map(([v, l]) => (
          <button key={v} className={preset === v ? 'btn-accent' : ''} onClick={() => pilih(v)}>{l}</button>
        ))}
        {preset === 'kustom' && (
          <span style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
            <input type="date" value={dari} onChange={(e) => setDari(e.target.value)} />
            <span className="muted">s/d</span>
            <input type="date" value={sampai} onChange={(e) => setSampai(e.target.value)} />
          </span>
        )}
        <button className="btn-accent" onClick={ekspor} disabled={eksporSibuk}>
          {eksporSibuk ? 'Menyiapkan file…' : '⬇ Ekspor Excel'}
        </button>
      </div>
      {pesan && <p className={pesan.ok ? 'success-text' : 'danger-text'}>{pesan.text}</p>}

      {summary && (
        <div className="stat-cards" style={{ marginBottom: 16 }}>
          <div className="card"><h3>Omzet</h3><div className="val">{rupiah(summary.omzet)}</div></div>
          <div className="card"><h3>HPP</h3><div className="val">{rupiah(summary.hpp)}</div></div>
          <div className="card"><h3>Laba kotor</h3><div className="val">{rupiah(summary.laba_kotor)}</div></div>
          <div className="card"><h3>Pengeluaran</h3><div className="val">{rupiah(summary.pengeluaran)}</div></div>
          <div className="card"><h3>Laba bersih</h3><div className="val">{rupiah(summary.laba_bersih)}</div></div>
          <div className="card"><h3>Transaksi</h3><div className="val">{summary.jumlah_transaksi}</div></div>
          <div className="card"><h3>Rata-rata/transaksi</h3><div className="val">{rupiah(summary.rata_rata_transaksi)}</div></div>
        </div>
      )}

      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Tren harian</h3>
          <ResponsiveContainer width="100%" height={240}>
            <LineChart data={trend?.data || []}>
              <XAxis dataKey="tanggal" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip formatter={(v) => rupiah(v)} />
              <Line type="monotone" dataKey="omzet" className="chart-omzet" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Barang terlaris (laba)</h3>
          <ResponsiveContainer width="100%" height={240}>
            <BarChart data={(top?.berdasar_laba || []).map((x) => ({ nama: x.nama, laba: x.laba }))} layout="vertical">
              <XAxis type="number" hide />
              <YAxis type="category" dataKey="nama" width={110} tick={{ fontSize: 11 }} />
              <Tooltip formatter={(v) => rupiah(v)} />
              <Bar dataKey="laba" className="chart-laba" radius={[0, 6, 6, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="card" style={{ marginTop: 14 }}>
        <h3 style={{ marginTop: 0 }}>Terlaris</h3>
        <div className="grid-2">
          <div>
            <p className="muted">Berdasar jumlah</p>
            <div className="table-wrap"><table className="lb">
              <thead><tr><th>Produk</th><th>Qty</th></tr></thead>
              <tbody>{(top?.berdasar_jumlah || []).map((x) => (
                <tr key={x.nama}><td>{x.nama}</td><td>{x.qty}</td></tr>
              ))}</tbody>
            </table></div>
          </div>
          <div>
            <p className="muted">Berdasar laba</p>
            <div className="table-wrap"><table className="lb">
              <thead><tr><th>Produk</th><th>Laba</th></tr></thead>
              <tbody>{(top?.berdasar_laba || []).map((x) => (
                <tr key={x.nama}><td>{x.nama}</td><td>{rupiah(x.laba)}</td></tr>
              ))}</tbody>
            </table></div>
          </div>
        </div>
      </div>
    </>
  )
}
