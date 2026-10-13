import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

export default function Pengeluaran() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const [form, setForm] = useState({ tanggal: '', expense_category_id: '', jumlah: '', metode: 'tunai', catatan: '' })
  const [msg, setMsg] = useState('')

  const { data: kats } = useQuery({ queryKey: ['exp-cats'], queryFn: () => api('/expense-categories') })
  const { data } = useQuery({ queryKey: ['expenses'], queryFn: () => api('/expenses?per_page=50') })

  const simpan = useMutation({
    mutationFn: () => api('/expenses', {
      method: 'POST',
      body: { ...form, expense_category_id: Number(form.expense_category_id), jumlah: Number(form.jumlah) },
    }),
    onSuccess: () => {
      setMsg('Pengeluaran dicatat.')
      setForm({ tanggal: '', expense_category_id: '', jumlah: '', metode: 'tunai', catatan: '' })
      qc.invalidateQueries()
    },
    onError: (e) => setMsg(e.message),
  })

  const hapus = useMutation({
    mutationFn: (id) => api('/expenses/' + id, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries(),
    onError: (e) => setMsg(e.message),
  })

  return (
    <>
      <h1>Pengeluaran</h1>
      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Catat cepat</h3>
          <div style={{ display: 'grid', gap: 8 }}>
            <input type="date" value={form.tanggal} onChange={(e) => setForm({ ...form, tanggal: e.target.value })} />
            <select value={form.expense_category_id} onChange={(e) => setForm({ ...form, expense_category_id: e.target.value })}>
              <option value="">— Kategori —</option>
              {(kats?.data || []).map((k) => <option key={k.id} value={k.id}>{k.nama}</option>)}
            </select>
            <input type="number" placeholder="Nominal Rp" value={form.jumlah}
              onChange={(e) => setForm({ ...form, jumlah: e.target.value })} />
            <select value={form.metode} onChange={(e) => setForm({ ...form, metode: e.target.value })}>
              <option value="tunai">Tunai</option>
              <option value="non_tunai">Non-tunai</option>
            </select>
            <input placeholder="Catatan" value={form.catatan}
              onChange={(e) => setForm({ ...form, catatan: e.target.value })} />
            <button className="btn-accent" disabled={simpan.isPending || !form.expense_category_id || !form.jumlah}
              onClick={() => simpan.mutate()}>Simpan</button>
          </div>
          {msg && <p className="muted">{msg}</p>}
        </div>
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Daftar</h3>
          <div className="table-wrap"><table className="lb">
            <thead><tr><th>Tanggal</th><th>Kategori</th><th>Nominal</th><th>Metode</th><th>Catatan</th>{isPemilik && <th></th>}</tr></thead>
            <tbody>
              {(data?.data || []).map((e) => (
                <tr key={e.id}>
                  <td>{e.tanggal}</td>
                  <td>{e.category_nama}</td>
                  <td>{rupiah(e.jumlah)}</td>
                  <td>{e.metode}</td>
                  <td>{e.catatan || '-'}</td>
                  {isPemilik && <td><button className="btn-danger" onClick={() => hapus.mutate(e.id)}>Hapus</button></td>}
                </tr>
              ))}
            </tbody>
          </table></div>
        </div>
      </div>
    </>
  )
}
