import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'

export default function Pengguna() {
  const qc = useQueryClient()
  const [form, setForm] = useState({ nama: '', username: '', password: '', peran: 'kasir' })
  const [msg, setMsg] = useState('')
  const { data: users } = useQuery({ queryKey: ['users'], queryFn: () => api('/users') })
  const { data: setts, refetch } = useQuery({ queryKey: ['settings'], queryFn: () => api('/settings') })

  const tambah = useMutation({
    mutationFn: () => api('/users', { method: 'POST', body: form }),
    onSuccess: () => { setMsg('Pengguna dibuat.'); setForm({ nama: '', username: '', password: '', peran: 'kasir' }); qc.invalidateQueries() },
    onError: (e) => setMsg(e.message),
  })
  const nonaktif = useMutation({
    mutationFn: (u) => api('/users/' + u.id, { method: 'PUT', body: { aktif: false } }),
    onSuccess: () => { qc.invalidateQueries(); setMsg('Pengguna dinonaktifkan — token lama langsung ditolak.') },
    onError: (e) => setMsg(e.message),
  })
  const reset = useMutation({
    mutationFn: (u) => api(`/users/${u.id}/reset-password`, { method: 'POST', body: {} }),
    onSuccess: (r) => { setMsg(`Password baru: ${r.password_baru} (beri ke pengguna)`); },
    onError: (e) => setMsg(e.message),
  })
  const simpanSetts = useMutation({
    mutationFn: () => api('/settings', { method: 'PUT', body: setts }),
    onSuccess: () => { setMsg('Pengaturan disimpan.'); refetch() },
  })

  return (
    <>
      <h1>Pengguna & Pengaturan</h1>
      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Tambah pengguna</h3>
          <div style={{ display: 'grid', gap: 8 }}>
            <input placeholder="Nama" value={form.nama} onChange={(e) => setForm({ ...form, nama: e.target.value })} />
            <input placeholder="Username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            <input placeholder="Password (min 6)" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            <select value={form.peran} onChange={(e) => setForm({ ...form, peran: e.target.value })}>
              <option value="kasir">kasir</option>
              <option value="pemilik">pemilik</option>
            </select>
            <button className="btn-accent" onClick={() => tambah.mutate()}>Simpan</button>
          </div>
        </div>
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Daftar pengguna</h3>
          <div className="table-wrap"><table className="lb">
            <thead><tr><th>Nama</th><th>Username</th><th>Peran</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {(users?.data || []).map((u) => (
                <tr key={u.id} style={{ opacity: u.aktif ? 1 : 0.5 }}>
                  <td>{u.nama}</td>
                  <td>{u.username}</td>
                  <td>{u.peran}</td>
                  <td>{u.aktif ? 'aktif' : 'nonaktif'}</td>
                  <td className="row-actions">
                    {u.aktif && <button onClick={() => nonaktif.mutate(u)}>Nonaktifkan</button>}
                    <button onClick={() => reset.mutate(u)}>Reset Password</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table></div>
        </div>
      </div>

      <div className="card" style={{ marginTop: 14 }}>
        <h3 style={{ marginTop: 0 }}>Pengaturan</h3>
        {setts && (
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
            <label>
              <input type="checkbox" checked={setts.izin_stok_negatif === '1'}
                onChange={(e) => api('/settings', { method: 'PUT', body: { izin_stok_negatif: e.target.checked ? '1' : '0' } }).then(refetch)} />
              {' '}Izinkan stok negatif
            </label>
            <label style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
              Zona waktu: <input value={setts.zona_waktu} onChange={(e) => setts.zona_waktu = e.target.value} style={{ width: 160 }} />
            </label>
            <button className="btn-accent" onClick={() => simpanSetts.mutate()}>Simpan Pengaturan</button>
          </div>
        )}
      </div>
      {msg && <p className="muted" style={{ marginTop: 10 }}>{msg}</p>}
    </>
  )
}
