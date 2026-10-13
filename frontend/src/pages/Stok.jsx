import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

// Istilah yang dibaca manusia (bukan kode internal)
const jenisManusia = {
  awal: 'Stok awal',
  masuk: 'Barang masuk',
  keluar: 'Barang keluar',
  koreksi: 'Koreksi (hitung ulang)',
  batal: 'Pembatalan transaksi',
}
const sumberManusia = {
  penjualan: 'Penjualan',
  penjualan_batal: 'Pembatalan penjualan',
  pembelian: 'Pembelian',
  pembelian_batal: 'Pembatalan pembelian',
  koreksi: 'Hasil hitung ulang',
  awal: 'Stock awal',
}
const sumberTeks = (m) => {
  const dasar = sumberManusia[m.referensi_tipe] || m.referensi_tipe || '-'
  return m.referensi_id ? `${dasar} nomor ${m.referensi_id}` : dasar
}
const jamMenit = (iso) => (iso || '').replace('T', ' ').slice(0, 16).replace('Z', '')

export default function Stok() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const page = 1
  const [jawikan, setJawi] = useState('') // filter jenis pergerakan
  const [pilihBarang, setPilihBarang] = useState('') // id barang yang dipakai kartu stok
  const [adj, setAdj] = useState({ product_id: '', stok_baru: '', alasan: '' })
  const [pesan, setPesan] = useState('')

  const { data: daftarB } = useQuery({
    queryKey: ['stok-daftar-barang'],
    queryFn: () => api('/products?per_page=200'),
  })
  const barang = daftarB?.data || []
  const namaBarang = (id) => barang.find((b) => b.id === Number(id))?.nama || 'Barang ' + id

  const { data: pergerakan } = useQuery({
    queryKey: ['movements', jawikan],
    queryFn: () => api(`/stock/movements?page=${page}${jawikan ? '&jenis=' + jawikan : ''}&per_page=50`),
    placeholderData: (p) => p,
  })
  const { data: cocok } = useQuery({
    queryKey: ['konsistensi'],
    queryFn: () => api('/stock/consistency'),
  })
  const { data: kartu } = useQuery({
    queryKey: ['kartu-stok', pilihBarang],
    queryFn: () => api(`/products/${pilihBarang}/stock-card`),
    enabled: !!pilihBarang,
  })

  const barangPilihan = barang.find((b) => b.id === Number(pilihBarang))

  const koreksi = useMutation({
    mutationFn: () => api('/stock/adjust', {
      method: 'POST',
      body: { product_id: Number(adj.product_id), stok_baru: Number(adj.stok_baru), alasan: adj.alasan },
    }),
    onSuccess: () => { setPesan('Stok barang berhasil disesuaikan.'); setAdj({ product_id: '', stok_baru: '', alasan: '' }); qc.invalidateQueries() },
    onError: (e) => setPesan(e.message),
  })

  const pilihDrop = (nilai, saatGanti) => (
    <select value={nilai} onChange={(e) => saatGanti(e.target.value)} style={{ width: '100%' }}>
      <option value="">— Pilih barang —</option>
      {barang.map((b) => (
        <option key={b.id} value={b.id}>{b.nama} (sisa {b.stok})</option>
      ))}
    </select>
  )

  return (
    <>
      <h1>Stok</h1>

      {/* Pemeriksaan otomatis, kata-kata manusia */}
      {cocok && (
        cocok.konsisten ? (
          <p className="success-text" style={{ marginBottom: 10 }}>✓ Semua stok cocok dengan riwayatnya — catatan aman.</p>
        ) : (
          <p className="warning-text" style={{ marginBottom: 10 }}>
            ⚠ Ada stok yang tidak cocok dengan riwayat: {(cocok.produk_tidak_cocok || []).map((id) => namaBarang(id)).join(', ')}. Coba hitung ulang di bawah.
          </p>
        )
      )}

      {/* Kartu stok: pilih barang dengan NAMA, tampil angka besar */}
      <div className="card" style={{ marginBottom: 14 }}>
        <h3 style={{ marginTop: 0 }}>Lihat Stok Barang</h3>
        {pilihDrop(pilihBarang, setPilihBarang)}
        {barangPilihan && (
          <div className="stat-cards" style={{ marginTop: 10 }}>
            <div className="card"><h3>{barangPilihan.nama} — sisa sekarang</h3><div className="val">{barangPilihan.stok} {barangPilihan.satuan}</div></div>
            <div className="card"><h3>Batas minimum</h3><div className="val">{barangPilihan.stok_minimum}</div></div>
            <div className="card"><h3>Total nilai barang ini</h3><div className="val">{rupiah(barangPilihan.stok * barangPilihan.harga_beli)}</div></div>
          </div>
        )}

        {pilihBarang && kartu && (
          <>
            <h3 style={{ marginTop: 14 }}>Riwayat {barangPilihan?.nama}</h3>
            <div className="table-wrap"><table className="lb">
              <thead><tr><th>Waktu</th><th>Kejadian</th><th>Jumlah</th><th>Sisa Setelah Ini</th><th>Dari Mana</th></tr></thead>
              <tbody>
                {(kartu.data || []).map((m) => (
                  <tr key={m.id}>
                    <td>{jamMenit(m.created_at)}</td>
                    <td>{jenisManusia[m.jenis] || m.jenis}</td>
                    <td className={m.jumlah < 0 ? 'danger-text' : 'success-text'}>{m.jumlah > 0 ? '+' : ''}{m.jumlah}</td>
                    <td><b>{m.saldo}</b></td>
                    <td>{sumberTeks(m)}</td>
                  </tr>
                ))}
              </tbody>
            </table></div>
          </>
        )}
        {!pilihBarang && <p className="muted" style={{ marginTop: 8 }}>Pilih barang di atas untuk melihat riwayat lengkapnya.</p>}

        {isPemilik && (
          <div style={{ marginTop: 12 }}>
            <h3 style={{ marginBottom: 8 }}>Hitung ulang stok (hasil opname) — hanya pemilik, wajib alasan</h3>
            <div className="form-row" style={{ gap: 8 }}>
              {pilihDrop(adj.product_id || '', (v) => setAdj({ ...adj, product_id: v }))}
              <input placeholder="Hasil hitungan fisik di rak" type="number" value={adj.stok_baru}
                onChange={(e) => setAdj({ ...adj, stok_baru: e.target.value })} style={{ width: 200 }} />
              <input placeholder="Alasan (mis. stok rusak)" value={adj.alasan}
                onChange={(e) => setAdj({ ...adj, alasan: e.target.value })} style={{ flex: 1, minWidth: 180 }} />
              <button className="btn-accent" onClick={() => koreksi.mutate()}>Simpan Koreksi</button>
            </div>
            {pesan && <p className={pesan.includes('berhasil') ? 'success-text' : 'danger-text'} style={{ marginTop: 8 }}>{pesan}</p>}
          </div>
        )}
      </div>

      {/* Riwayat semua pergerakan */}
      <div className="card">
        <h3 style={{ marginTop: 0 }}>Riwayat Semua Pergerakan Barang</h3>
        <div style={{ display: 'flex', gap: 8, marginBottom: 10, flexWrap: 'wrap' }}>
          <select value={jawikan} onChange={(e) => setJawi(e.target.value)} style={{ width: 240 }}>
            <option value="">Semua kejadian</option>
            {Object.entries(jenisManusia).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
          </select>
        </div>
        <div className="table-wrap"><table className="lb">
          <thead><tr><th>Waktu</th><th>Nama Barang</th><th>Kejadian</th><th>Jumlah</th><th>Dari Mana</th><th>Keterangan</th></tr></thead>
          <tbody>
            {(pergerakan?.data || []).map((m) => (
              <tr key={m.id}>
                <td>{jamMenit(m.created_at)}</td>
                <td>{m.product_nama}</td>
                <td>{jenisManusia[m.jenis] || m.jenis}</td>
                <td className={m.jumlah < 0 ? 'danger-text' : 'success-text'}>{m.jumlah > 0 ? '+' : ''}{m.jumlah}</td>
                <td>{sumberTeks(m)}</td>
                <td>{m.catatan || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table></div>
        <div className="row-actions" style={{ marginTop: 10 }}>
          <span className="muted">Menampilkan {(pergerakan?.data || []).length} dari {pergerakan?.meta?.total ?? 0} kejadian terakhir</span>
        </div>
      </div>
    </>
  )
}
