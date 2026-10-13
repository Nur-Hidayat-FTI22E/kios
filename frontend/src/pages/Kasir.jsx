import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

export default function Kasir() {
  const qc = useQueryClient()
  const searchRef = useRef(null)
  const [q, setQ] = useState('')
  const [cart, setCart] = useState([]) // {product_id, nama, satuan, jumlah, harga, stok}
  const [diskon, setDiskon] = useState(0)
  const [metode, setMetode] = useState('tunai')
  const [dibayar, setDibayar] = useState('')
  const [customer, setCustomer] = useState(null)
  const [custQ, setCustQ] = useState('')
  const [newCust, setNewCust] = useState('')
  const [msg, setMsg] = useState(null) // {ok, text, extra}
  const [busy, setBusy] = useState(false)

  // hasil pencarian muncul seketika (12.2)
  const { data: hasil } = useQuery({
    queryKey: ['pos-cari', q],
    queryFn: () => api('/products?q=' + encodeURIComponent(q) + '&per_page=20'),
    enabled: true,
    placeholderData: (prev) => prev,
  })
  const { data: favorit } = useQuery({
    queryKey: ['pos-favorit'],
    queryFn: () => api('/products?favorit=1&per_page=20'),
  })
  const { data: custs } = useQuery({
    queryKey: ['pos-cust', custQ],
    queryFn: () => api('/customers?q=' + encodeURIComponent(custQ)),
    enabled: metode === 'kasbon',
  })

  useEffect(() => { searchRef.current?.focus() }, [])

  // Enter menambah item pertama (12.2)
  const pilih = (p) => {
    setMsg(null)
    setCart((c) => {
      const i = c.findIndex((x) => x.product_id === p.id)
      if (i >= 0) {
        const cp = [...c]
        cp[i] = { ...cp[i], jumlah: cp[i].jumlah + 1 }
        return cp
      }
      return [...c, { product_id: p.id, nama: p.nama, satuan: p.satuan, jumlah: 1, harga: p.harga_jual, hargaAsli: p.harga_jual, stok: p.stok }]
    })
  }

  const subtotal = useMemo(() => cart.reduce((s, x) => s + x.harga * x.jumlah, 0), [cart])
  const total = Math.max(0, subtotal - (Number(diskon) || 0))
  const kembalian = metode === 'tunai' && dibayar !== '' ? Math.max(0, (Number(dibayar) || 0) - total) : 0
  const kurangBayar = metode === 'tunai' && dibayar !== '' && (Number(dibayar) || 0) < total

  const ubahJml = (idx, d) => {
    setCart((c) => {
      const cp = [...c]
      const j = cp[idx].jumlah + d
      if (j <= 0) cp.splice(idx, 1)
      else cp[idx] = { ...cp[idx], jumlah: j }
      return cp
    })
  }
  const ubahHarga = (idx, v) => {
    setCart((c) => { const cp = [...c]; cp[idx] = { ...cp[idx], harga: Math.max(0, Number(v) || 0) }; return cp })
  }

  const buatPelangganCepat = async () => {
    if (!newCust.trim()) return
    const r = await api('/customers', { method: 'POST', body: { nama: newCust.trim() } })
    setCustomer({ id: r.id, nama: newCust.trim() })
    setNewCust('')
  }

  // POST /sales dengan idempotency key (FR-POS-09)
  const bayar = async () => {
    if (cart.length === 0 || busy) return
    setBusy(true)
    setMsg(null)
    const body = {
      items: cart.map((x) => ({ product_id: x.product_id, jumlah: x.jumlah, harga_jual: x.harga })),
      diskon: Number(diskon) || 0,
      metode,
      dibayar: metode === 'kasbon' ? (Number(dibayar) || 0) : (metode === 'qris' ? 0 : (Number(dibayar) || 0)),
      customer_id: metode === 'kasbon' ? customer?.id : undefined,
    }
    if (metode === 'qris') body.dibayar = total
    try {
      const r = await api('/sales', { method: 'POST', body, idempotent: true })
      setMsg({
        ok: true,
        text: `Transaksi ${r.nomor} tersimpan — total ${rupiah(r.total)}` +
          (r.kembalian ? `, kembalian ${rupiah(r.kembalian)}` : '') +
          (r.kasbon_sisa ? `, sisa kasbon ${rupiah(r.kasbon_sisa)}` : ''),
      })
      setCart([]); setDiskon(''); setDibayar(''); setMetode('tunai'); setCustomer(null)
      qc.invalidateQueries()
    } catch (e) {
      setMsg({ ok: false, text: e.message })
    } finally {
      setBusy(false)
    }
  }

  const tampilHasil = (q ? (hasil?.data ||[]) : (favorit?.data || []))

  return (
    <div className="pos-grid">
      <div className="card">
        <h2 style={{ marginTop: 0 }}>Kasir</h2>
        <input
          ref={searchRef}
          className="search-big"
          placeholder="Cari produk… (Enter = tambah item pertama)"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && tampilHasil[0]) pilih(tampilHasil[0]) }}
        />
        <div className="fav-row">
          {(favorit?.data || []).map((p) => (
            <button key={p.id} className="fav-btn" onClick={() => pilih(p)}>
              {p.nama}<br /><span className="muted">{rupiah(p.harga_jual)}</span>
            </button>
          ))}
          {(favorit?.data || []).length === 0 && <span className="muted">Belum ada produk favorit.</span>}
        </div>
        <div>
          {(q ? (hasil?.data || []) : []).slice(0, 8).map((p) => (
            <button key={p.id} style={{ width: '100%', marginBottom: 6, textAlign: 'left' }} onClick={() => pilih(p)}>
              {p.nama} · {p.satuan} · <b>{rupiah(p.harga_jual)}</b> <span className="muted">(stok {p.stok})</span>
            </button>
          ))}
        </div>

        <h3>Keranjang</h3>
        {cart.length === 0 && <p className="muted">Keranjang kosong.</p>}
        {cart.map((x, i) => (
          <div className="cart-item" key={x.product_id}>
            <div style={{ flex: 1 }}>
              <b>{x.nama}</b><br />
              <input
                type="number" min="0" value={x.harga}
                onChange={(e) => ubahHarga(i, e.target.value)}
                style={{ width: 100, padding: '4px 8px', minHeight: 36 }}
                title="Harga bisa diubah (tawar-menawar)"
              />
            </div>
            <div className="qty">
              <button onClick={() => ubahJml(i, -1)}>−</button>
              <span>{x.jumlah}</span>
              <button onClick={() => ubahJml(i, 1)}>+</button>
            </div>
            <div style={{ width: 110, textAlign: 'right' }}>{rupiah(x.harga * x.jumlah)}</div>
          </div>
        ))}
        {cart.length > 0 && (
          <>
            <div style={{ display: 'flex', gap: 10, marginTop: 10, alignItems: 'center', flexWrap: 'wrap' }}>
              <label className="muted">Diskon Rp</label>
              <input type="number" min="0" value={diskon} onChange={(e) => setDiskon(e.target.value)} style={{ width: 140 }} />
            </div>
            <div className="total-bar">Total: {rupiah(total)}</div>
          </>
        )}
      </div>

      <div className="card">
        <h3 style={{ marginTop: 0 }}>Pembayaran</h3>
        <div className="method-row">
          {['tunai', 'qris', 'kasbon'].map((m) => (
            <button key={m} className={metode === m ? 'selected' : ''} onClick={() => { setMetode(m); setDibayar('') }}>
              {m === 'tunai' ? 'Tunai' : m === 'qris' ? 'QRIS' : 'Kasbon'}
            </button>
          ))}
        </div>

        {metode === 'tunai' && (
          <>
            <div className="quickcash-row">
              <button onClick={() => setDibayar(total)}>Uang pas</button>
              {[20000, 50000, 100000].map((v) => (
                <button key={v} onClick={() => setDibayar((Number(dibayar) || 0) + v)}>{rupiah(v)}</button>
              ))}
            </div>
            <input type="number" placeholder="Uang diterima" value={dibayar}
              onChange={(e) => setDibayar(e.target.value)} style={{ width: '100%' }} />
            {dibayar !== '' && (
              <p className={kurangBayar ? 'danger-text' : 'success-text'} style={{ fontSize: 18 }}>
                {kurangBayar ? `Masih kurang ${rupiah(total - (Number(dibayar) || 0))}` : `Kembalian: ${rupiah(kembalian)}`}
              </p>
            )}
          </>
        )}

        {metode === 'kasbon' && (
          <>
            <input placeholder="Cari pelanggan…" value={custQ} onChange={(e) => setCustQ(e.target.value)} style={{ width: '100%' }} />
            <div style={{ maxHeight: 160, overflow: 'auto', margin: '8px 0' }}>
              {(custs?.data || []).map((c) => (
                <button key={c.id} style={{
                  width: '100%', marginBottom: 6, textAlign: 'left',
                  boxShadow: customer?.id === c.id ? 'var(--inset)' : 'var(--raised)',
                  color: customer?.id === c.id ? 'var(--accent)' : 'var(--text)',
                }} onClick={() => setCustomer(c)}>
                  {c.nama}{c.telepon ? ` · ${c.telepon}` : ''}
                </button>
              ))}
            </div>
            {/* FR-DBT-06: tambah pelanggan cepat tanpa keluar layar */}
            <div style={{ display: 'flex', gap: 8 }}>
              <input placeholder="Nama pelanggan baru" value={newCust} onChange={(e) => setNewCust(e.target.value)} style={{ flex: 1 }} />
              <button onClick={buatPelangganCepat}>Tambah</button>
            </div>
            {customer && <p className="success-text">Pelanggan: {customer.nama}</p>}
            <input type="number" placeholder="DP tunai (boleh 0)" value={dibayar}
              onChange={(e) => setDibayar(e.target.value)} style={{ width: '100%', marginTop: 8 }} />
          </>
        )}

        {metode === 'qris' && <p className="muted">QRIS dicatat lunas tanpa integrasi otomatis.</p>}

        <button className="btn-accent" style={{ width: '100%', fontSize: 20, minHeight: 56, marginTop: 10 }}
          disabled={cart.length === 0 || busy || !!(metode === 'tunai' && kurangBayar) || (metode === 'kasbon' && !customer)}
          onClick={bayar}>
          {busy ? 'Menyimpan…' : 'Bayar'}
        </button>
        {msg && <p className={msg.ok ? 'success-text' : 'danger-text'} style={{ marginTop: 10 }}>{msg.text}</p>}
      </div>
    </div>
  )
}
