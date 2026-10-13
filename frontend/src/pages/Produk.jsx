import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

export default function Produk() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const [q, setQ] = useState('')
  const [katFilter, setKatFilter] = useState('')
  const [namaKategoriBaru, setNamaKategoriBaru] = useState('') // state tombol + Kategori Baru
  const [form, setForm] = useState(null) // null | {} product fields
  const [csv, setCsv] = useState('')
  const [preview, setPreview] = useState(null)

  // Daftar penuh (sampai 200 barang) supaya bisa dikelompokkan per kategori
  const { data } = useQuery({
    queryKey: ['produk', q],
    queryFn: () => api(`/products?q=${encodeURIComponent(q)}&per_page=200`),
    placeholderData: (p) => p,
  })
  const { data: kats } = useQuery({ queryKey: ['kategori'], queryFn: () => api('/categories') })

  const buatKategori = useMutation({
    mutationFn: () => api('/categories', { method: 'POST', body: { nama: namaKategoriBaru.trim() } }),
    onSuccess: () => { setNamaKategoriBaru(''); qc.invalidateQueries({ queryKey: ['kategori'] }) },
    onError: (e) => alert(e.message),
  })

  const daftar = data?.data || []
  const tanpaKategori = '(Tanpa kategori)'
  const jumlahPerKategori = {}
  daftar.forEach((p) => {
    const nome = p.category_nama || tanpaKategori
    jumlahPerKategori[nome] = (jumlahPerKategori[nome] || 0) + 1
  })
  // Tab kategori diambil dari daftar kategori server (termasuk yang masih kosong) + kategori hasil impor
  const namaKategori = [...new Set([
    ...(kats?.data || []).map((k) => k.nama),
    ...Object.keys(jumlahPerKategori),
  ])].sort((a, b) => a.localeCompare(b))
  const grupTampil = daftar
    .filter((p) => katFilter === '' || (p.category_nama || tanpaKategori) === katFilter)
    .reduce((acc, p) => {
      const nome = p.category_nama || tanpaKategori
      ;(acc[nome] = acc[nome] || []).push(p)
      return acc
    }, {})

  const simpan = useMutation({
    mutationFn: async (body) => {
      const stokAwal = body.stok_awal
      delete body.stok_awal
      if (body.id) return api('/products/' + body.id, { method: 'PUT', body })
      return api('/products?stok_awal=' + (stokAwal || 0), { method: 'POST', body })
    },
    onSuccess: () => { setForm(null); qc.invalidateQueries() },
  })
  const toggleFav = useMutation({
    mutationFn: (p) => api('/products/' + p.id, { method: 'PUT', body: { favorit: !p.favorit } }),
    onSuccess: () => qc.invalidateQueries(),
  })
  const nonaktifkan = useMutation({
    mutationFn: (p) => api('/products/' + p.id, { method: 'PUT', body: { aktif: false } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['produk'] }),
    onError: (e) => alert(e.message),
  })
  const aktifkan = useMutation({
    mutationFn: (p) => api('/products/' + p.id, { method: 'PUT', body: { aktif: true } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['produk'] }),
    onError: (e) => alert(e.message),
  })

  const stokAwal = useMutation({
    mutationFn: () => api('/products/import/preview', { method: 'POST', body: { csv } }),
    onSuccess: (p) => setPreview(p),
  })
  const impor = useMutation({
    mutationFn: () => api('/products/import/commit', { method: 'POST', body: { csv } }),
    onSuccess: () => { setCsv(''); setPreview(null); qc.invalidateQueries(); alert('Impor berhasil') },
    onError: (e) => alert(e.message),
  })

  const barisBarang = (p) => (
    <tr key={p.id} style={{ opacity: p.aktif ? 1 : 0.5 }}>
      <td>
        {p.favorit ? '★ ' : ''}{p.nama}
        {!p.aktif && <span className="muted"> — tidak dijual</span>}
      </td>
      <td>{p.satuan}</td>
      {isPemilik && <td>{rupiah(p.harga_beli)}</td>}
      <td><b>{rupiah(p.harga_jual)}</b></td>
      <td className={p.stok <= p.stok_minimum && p.stok_minimum > 0 ? 'warning-text' : ''}>
        {p.stok}{p.stok_minimum > 0 && p.stok <= p.stok_minimum ? ' ⚠' : ''}
      </td>
      <td className="row-actions">
        <button onClick={() => toggleFav.mutate(p)} title="Tampilkan di layar Kasir">{p.favorit ? '★ Favorit' : '☆ Favorit'}</button>
        {isPemilik && <button onClick={() => setForm(p)}>Ubah</button>}
        {isPemilik && p.aktif && <button className="btn-danger" onClick={() => nonaktifkan.mutate(p)}>Nonaktif</button>}
        {isPemilik && !p.aktif && <button className="btn-success" onClick={() => aktifkan.mutate(p)}>Aktifkan</button>}
      </td>
    </tr>
  )

  return (
    <>
      <h1>Produk</h1>
      <div style={{ display: 'flex', gap: 10, marginBottom: 12, flexWrap: 'wrap' }}>
        <input placeholder="Cari nama barang…" value={q} onChange={(e) => setQ(e.target.value)} style={{ flex: 1, minWidth: 200 }} />
        {isPemilik && <button onClick={() => setForm({})}>+ Barang Baru</button>}
        {isPemilik && <button onClick={() => setForm('impor')}>Impor CSV</button>}
        {isPemilik && (namaKategoriBaru === '' ? (
          <button onClick={() => setNamaKategoriBaru(' ')}>+ Kategori Baru</button>
        ) : (
          <div className="form-row" style={{ gap: 6 }}>
            <input placeholder="Nama kategori" value={namaKategoriBaru === ' ' ? '' : namaKategoriBaru}
              autoFocus
              onChange={(e) => setNamaKategoriBaru(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && namaKategoriBaru.trim()) buatKategori.mutate() }}
              style={{ width: 160 }} />
            <button className="btn-accent" disabled={!namaKategoriBaru.trim()} onClick={() => buatKategori.mutate()}>Buat Kategori</button>
            <button onClick={() => setNamaKategoriBaru('')}>Batal</button>
          </div>
        ))}
      </div>

      {form === 'impor' && (
        <div className="card" style={{ marginBottom: 14 }}>
          <h3 style={{ marginTop: 0 }}>Impor CSV/Excel (kolom: nama, kategori, satuan, harga_beli, harga_jual, stok_awal, stok_minimum)</h3>
          <textarea value={csv} onChange={(e) => setCsv(e.target.value)} rows={6} style={{ width: '100%', fontFamily: 'monospace' }}
            placeholder={'Bakso Goreng, Makanan, pcs, 2500, 5000, 100, 20'} />
          <div className="row-actions" style={{ marginTop: 10 }}>
            <button onClick={() => stokAwal.mutate()}>Pratinjau</button>
            {preview && <button className="btn-accent" onClick={() => impor.mutate()}>Impor Semua (semua-atau-tidak)</button>}
          </div>
          {preview && (
            <div className="table-wrap"><table className="lb" style={{ marginTop: 10 }}>
              <thead><tr><th>Baris</th><th>Nama</th><th>H. Beli</th><th>H. Jual</th><th>Stok</th></tr></thead>
              <tbody>
                {(preview.preview || []).map((r, i) => (
                  <tr key={i}>
                    <td>{r.baris}</td>
                    <td>{r.nama || <span className="danger-text">{r.galat || 'galat'}</span>}</td>
                    <td>{rupiah(r.harga_beli || 0)}</td>
                    <td>{rupiah(r.harga_jual || 0)}</td>
                    <td>{r.stok_awal ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table></div>
          )}
        </div>
      )}

      {form && form !== 'impor' && (
        <div className="card" style={{ marginBottom: 14 }}>
          <h3 style={{ marginTop: 0 }}>{form.id ? 'Ubah Barang' : 'Barang Baru'}</h3>
          <div className="grid-2">
            <input placeholder="Nama barang" value={form.nama || ''} onChange={(e) => setForm({ ...form, nama: e.target.value })} />
            <select value={form.category_id || ''} onChange={(e) => setForm({ ...form, category_id: Number(e.target.value) })}>
              <option value="">— Kategori —</option>
              {(kats?.data || []).map((k) => <option key={k.id} value={k.id}>{k.nama}</option>)}
            </select>
            <input placeholder="Satuan (pcs)" value={form.satuan || ''} onChange={(e) => setForm({ ...form, satuan: e.target.value })} />
            {isPemilik && <input placeholder="Harga beli" type="number" value={form.harga_beli ?? ''} onChange={(e) => setForm({ ...form, harga_beli: Number(e.target.value) })} />}
            {isPemilik && form.harga_beli > 0 && form.harga_jual < form.harga_beli && (
              <p className="warning-text">Harga jual lebih kecil dari harga beli!</p>
            )}
            <input placeholder="Harga jual" type="number" value={form.harga_jual ?? ''} onChange={(e) => setForm({ ...form, harga_jual: Number(e.target.value) })} />
            <input placeholder="Stok minimum" type="number" value={form.stok_minimum ?? ''} onChange={(e) => setForm({ ...form, stok_minimum: Number(e.target.value) })} />
            {!form.id && <input placeholder="Stok awal" type="number" value={form.stok_awal ?? ''} onChange={(e) => setForm({ ...form, stok_awal: Number(e.target.value) })} />}
          </div>
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn-accent" onClick={() => simpan.mutate(form)}>Simpan</button>
            <button onClick={() => setForm(null)}>Batal</button>
          </div>
          {simpan.isError && <p className="danger-text">{simpan.error.message}</p>}
        </div>
      )}

      {/* Filter kategori */}
      <div className="fav-row" style={{ marginBottom: 8 }}>
        <button className={katFilter === '' ? 'btn-accent' : ''} onClick={() => setKatFilter('')}>
          Semua ({daftar.length})
        </button>
        {namaKategori.map((kNome) => (
          <button key={kNome} className={katFilter === kNome ? 'btn-accent fav-btn' : 'fav-btn'} onClick={() => setKatFilter(kNome)}>
            {kNome} ({jumlahPerKategori[kNome] || 0})
          </button>
        ))}
      </div>

      {daftar.length === 0 && <div className="card muted">Belum ada barang {(q ? ' yang cocok dengan pencarian.' : '.')} {isPemilik && 'Klik "+ Barang Baru" untuk menambah.'}</div>}

      {Object.keys(grupTampil)
        .sort((a, b) => (a === tanpaKategori ? 1 : b === tanpaKategori ? -1 : a.localeCompare(b)))
        .map((nome) => (
          <div className="card" style={{ marginBottom: 14 }} key={nome}>
            <h3 style={{ marginTop: 0 }}>
              {nome} <span className="muted">({grupTampil[nome].length} barang)</span>
            </h3>
            <div className="table-wrap"><table className="lb">
              <thead>
                <tr><th>Nama Barang</th><th>Satuan</th>{isPemilik && <th>Harga Beli</th>}<th>Harga Jual</th><th>Stok</th><th></th></tr>
              </thead>
              <tbody>{grupTampil[nome].map(barisBarang)}</tbody>
            </table></div>
          </div>
        ))}
    </>
  )
}
