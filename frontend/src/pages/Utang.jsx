import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

export default function Utang() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const [tab, setTab] = useState('pelanggan')
  const [statusFilter, setStatusFilter] = useState('aktif')
  const [payForm, setPayForm] = useState(null) // {id, jumlah, metode}
  const [msg, setMsg] = useState('')

  const { data } = useQuery({
    queryKey: ['debts', tab, statusFilter],
    queryFn: () => api(`/debts?tipe=${tab}&status=${statusFilter}`),
    placeholderData: (p) => p,
  })
  const { data: summary } = useQuery({ queryKey: ['debt-summary'], queryFn: () => api('/debts/summary') })

  const bayar = useMutation({
    mutationFn: () => api(`/debts/${payForm.id}/payments`, {
      method: 'POST',
      body: { jumlah: Number(payForm.jumlah), metode: payForm.metode },
    }),
    onSuccess: (r) => {
      setMsg(r.lunas ? 'Utang lunas.' : `Dibayar, sisa ${rupiah(r.sisa)}.`)
      setPayForm(null)
      qc.invalidateQueries()
    },
    onError: (e) => setMsg(e.message),
  })

  const batalBayar = useMutation({
    mutationFn: (id) => {
      if (!confirm('Batalkan pembayaran ini?')) return Promise.reject(new Error('Dibatalkan'))
      return api(`/debt-payments/${id}/cancel`, { method: 'POST' })
    },
    onSuccess: () => { qc.invalidateQueries(); setMsg('Pembayaran dibatalkan.') },
    onError: (e) => setMsg(e.message),
  })

  return (
    <>
      <h1>Utang</h1>
      <div style={{ display: 'flex', gap: 10, marginBottom: 12, flexWrap: 'wrap' }}>
        <button className={tab === 'pelanggan' ? 'btn-accent' : ''} onClick={() => setTab('pelanggan')}>Pelanggan</button>
        <button className={tab === 'supplier' ? 'btn-accent' : ''} onClick={() => setTab('supplier')}>Supplier</button>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="aktif">Belum lunas</option>
          <option value="lunas">Lunas</option>
        </select>
      </div>

      {summary && (
        <div className="stat-cards" style={{ marginBottom: 14 }}>
          {tab === 'pelanggan'
            ? (summary.pelanggan || []).map((s) => (
              <div key={s.nama} className="card"><h3>{s.nama}</h3><div className="val">{rupiah(s.total_sisa)}</div></div>
            ))
            : (summary.supplier || []).map((s) => (
              <div key={s.nama} className="card"><h3>{s.nama}</h3><div className="val">{rupiah(s.total_sisa)}</div></div>
            ))}
          {((tab === 'pelanggan' ? summary.pelanggan : summary.supplier) || []).length === 0 && (
            <div className="card muted">Tidak ada utang {tab}. </div>
          )}
        </div>
      )}

      <div className="card">
        <div className="table-wrap"><table className="lb">
          <thead><tr><th>Pihak</th><th>Tanggal</th><th>Jumlah</th><th>Terbayar</th><th>Sisa</th><th>Jatuh Tempo</th><th>Status</th><th></th></tr></thead>
          <tbody>
            {(data?.data || []).map((d) => (
              <tr key={d.id}>
                <td>{d.pihak || '(tanpa nama)'}</td>
                <td>{(d.created_at || '').slice(0, 10)}</td>
                <td>{rupiah(d.jumlah)}</td>
                <td>{rupiah(d.terbayar)}</td>
                <td><b>{rupiah(d.sisa)}</b></td>
                <td>{d.jatuh_tempo || '-'}</td>
                <td>{d.lunas ? <span className="success-text">lunas</span> : d.status === 'batal' ? <span className="danger-text">batal</span> : <span className="warning-text">aktif</span>}</td>
                <td className="row-actions">
                  {!d.lunas && d.status === 'aktif' && (
                    <button className="btn-accent" onClick={() => setPayForm({ id: d.id, jumlah: d.sisa, metode: 'tunai' })}>Bayar</button>
                  )}
                  {isPemilik && (
                    <span className="muted" style={{ fontSize: 12 }}>hist. di detail</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table></div>
        {msg && <p className="muted" style={{ marginTop: 8 }}>{msg}</p>}
      </div>

      {payForm && (
        <div className="card" style={{ marginTop: 14 }}>
          <h3 style={{ marginTop: 0 }}>Catat pembayaran utang #{payForm.id}</h3>
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
            <label className="muted">Nominal Rp</label>
            <input type="number" min="1" value={payForm.jumlah}
              onChange={(e) => setPayForm({ ...payForm, jumlah: e.target.value })} style={{ width: 140 }} />
            <select value={payForm.metode} onChange={(e) => setPayForm({ ...payForm, metode: e.target.value })}>
              <option value="tunai">Tunai</option>
              <option value="qris">QRIS</option>
              <option value="transfer">Transfer</option>
            </select>
            <button className="btn-accent" onClick={() => bayar.mutate()} disabled={bayar.isPending}>Simpan</button>
            <button onClick={() => setPayForm(null)}>Batal</button>
          </div>
          {bayar.isError && <p className="danger-text">{bayar.error.message}</p>}
        </div>
      )}
    </>
  )
}
