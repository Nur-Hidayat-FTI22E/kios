import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

export default function Pembelian() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const [form, setForm] = useState({ supplier_id: '', metode_bayar: 'tunai', dibayar: '', catatan: '' })
  const [items, setItems] = useState([]) // {product_id, nama, jumlah, harga_beli}
  const [carProd, setCarProd] = useState('')
  const [msg, setMsg] = useState(null)

  const { data: supliers } = useQuery({ queryKey: ['suppliers'], queryFn: () => api('/suppliers') })
  const { data: prodCari } = useQuery({
    queryKey: ['beli-cari', carProd],
    queryFn: () => api('/products?q=' + encodeURIComponent(carProd) + '&per_page=8'),
    enabled: carProd.length > 0,
  })
  const { data: riwayat } = useQuery({ queryKey: ['purchases'], queryFn: () => api('/purchases') })

  const total = items.reduce((s, x) => s + x.jumlah * x.harga_beli, 0)

  const tambah = (p) => {
    setItems((it) => {
      const i = it.findIndex((x) => x.product_id === p.id)
      if (i >= 0) { const cp = [...it]; cp[i] = { ...cp[i], jumlah: cp[i].jumlah + 1 }; return cp }
      return [...it, { product_id: p.id, nama: p.nama, jumlah: 1, harga_beli: p.harga_beli || 0 }]
    })
    setCarProd('')
  }

  const simpan = useMutation({
    mutationFn: () => api('/purchases', {
      method: 'POST',
      body: {
        supplier_id: form.supplier_id ? Number(form.supplier_id) : null,
        items: items.map((x) => ({ product_id: x.product_id, jumlah: x.jumlah, harga_beli: Number(x.harga_beli) })),
        dibayar: Number(form.dibayar) || 0,
        metode_bayar: form.metode_bayar,
        catatan: form.catatan,
        update_harga: true,
      },
    }),
    onSuccess: (r) => {
      setMsg({ ok: true, text: `Pembelian #${r.id} tersimpan — total ${rupiah(r.total)}${r.utang ? `, utang ${rupiah(r.utang)}` : ''}` })
      setItems([]); setForm({ supplier_id: '', metode_bayar: 'tunai', dibayar: '', catatan: '' })
      qc.invalidateQueries()
    },
    onError: (e) => setMsg({ ok: false, text: e.message }),
  })

  const voidPurchase = useMutation({
    mutationFn: (id) => {
      const alasan = prompt('Alasan batal pembelian (wajib):')
      if (!alasan) return Promise.reject(new Error('Dibatalkan'))
      return api(`/purchases/${id}/void`, { method: 'POST', body: { alasan } })
    },
    onSuccess: () => qc.invalidateQueries(),
    onError: (e) => alert(e.message),
  })

  return (
    <>
      <h1>Pembelian / Barang Masuk</h1>
      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Form barang masuk</h3>
          <select value={form.supplier_id} onChange={(e) => setForm({ ...form, supplier_id: e.target.value })} style={{ width: '100%', marginBottom: 8 }}>
            <option value="">— Supplier (opsional) —</option>
            {(supliers?.data || []).map((s) => <option key={s.id} value={s.id}>{s.nama}</option>)}
          </select>
          <input placeholder="Cari produk…"
            value={carProd} onChange={(e) => setCarProd(e.target.value)} style={{ width: '100%', marginBottom: 6 }} />
          {(prodCari?.data || []).slice(0, 5).map((p) => (
            <button key={p.id} style={{ width: '100%', marginBottom: 4, textAlign: 'left' }} onClick={() => tambah(p)}>
              {p.nama} {isPemilik && `(beli ${rupiah(p.harga_beli)})`}
            </button>
          ))}
          {items.map((x, i) => (
            <div key={x.product_id} className="cart-item">
              <div style={{ flex: 1 }}><b>{x.nama}</b></div>
              <input type="number" min="1" value={x.jumlah}
                onChange={(e) => { const cp = [...items]; cp[i] = { ...x, jumlah: Number(e.target.value) }; setItems(cp) }}
                style={{ width: 70 }} />
              <input type="number" min="0" value={x.harga_beli} title="Harga beli per unit"
                onChange={(e) => { const cp = [...items]; cp[i] = { ...x, harga_beli: Number(e.target.value) }; setItems(cp) }}
                style={{ width: 110 }} />
            </div>
          ))}
          <div className="total-bar">Total: {rupiah(total)}</div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
            <select value={form.metode_bayar} onChange={(e) => setForm({ ...form, metode_bayar: e.target.value })}>
              <option value="tunai">Tunai</option>
              <option value="non_tunai">Non-tunai</option>
            </select>
            <input type="number" placeholder="Dibayar" value={form.dibayar}
              onChange={(e) => setForm({ ...form, dibayar: e.target.value })} style={{ width: 120 }} />
          </div>
          {form.supplier_id && total > (Number(form.dibayar) || 0) && (
            <p className="warning-text">Sisa {rupiah(total - (Number(form.dibayar) || 0))} menjadi utang supplier.</p>
          )}
          <input placeholder="Catatan (opsional)" value={form.catatan}
            onChange={(e) => setForm({ ...form, catatan: e.target.value })} style={{ width: '100%', marginTop: 8 }} />
          <button className="btn-accent" style={{ width: '100%', marginTop: 10 }}
            disabled={items.length === 0 || simpan.isPending} onClick={() => simpan.mutate()}>
            Simpan Pembelian
          </button>
          {msg && <p className={msg.ok ? 'success-text' : 'danger-text'}>{msg.text}</p>}
        </div>

        <div className="card">
          <h3 style={{ marginTop: 0 }}>Riwayat</h3>
          <div className="table-wrap"><table className="lb">
            <thead><tr><th>Tanggal</th><th>Supplier</th><th>Total</th><th>Dibayar</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {(riwayat?.data || []).map((p) => (
                <tr key={p.id}>
                  <td>{(p.created_at || '').slice(0, 10)}</td>
                  <td>{p.supplier_nama || '-'}</td>
                  <td>{rupiah(p.total)}</td>
                  <td>{rupiah(p.dibayar)}</td>
                  <td>{p.status === 'selesai' ? <span className="success-text">selesai</span> : <span className="danger-text">batal</span>}</td>
                  <td>{isPemilik && p.status === 'selesai' && (
                    <button className="btn-danger" onClick={() => voidPurchase.mutate(p.id)}>Void</button>
                  )}</td>
                </tr>
              ))}
            </tbody>
          </table></div>
        </div>
      </div>
    </>
  )
}
