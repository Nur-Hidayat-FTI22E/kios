import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, rupiah } from '../api/client'
import { useIsPemilik } from '../auth'

function hariIni() {
  const loc = new Date()
  loc.setMinutes(loc.getMinutes() - loc.getTimezoneOffset())
  return loc.toISOString().slice(0, 10)
}

export default function TutupKasir() {
  const qc = useQueryClient()
  const isPemilik = useIsPemilik()
  const [tanggal, setTanggal] = useState(hariIni())
  const [modalAwal, setModalAwal] = useState('')
  const [kasFisik, setKasFisik] = useState('')
  const [catatan, setCatatan] = useState('')
  const [msg, setMsg] = useState('')

  const preview = useQuery({
    queryKey: ['closing-preview', tanggal, modalAwal],
    queryFn: () => api(`/cash-closing/preview?tanggal=${tanggal}${modalAwal !== '' ? '&modal_awal=' + modalAwal : ''}`),
  })

  useEffect(() => { if (preview.data && modalAwal === '') setModalAwal(String(preview.data.rincian?.modal_awal ?? 0)) }, [preview.data])

  const kasSistem = preview.data?.kas_sistem ?? 0
  const selisih = kasFisik === '' ? null : (Number(kasFisik) || 0) - kasSistem

  const simpan = useMutation({
    mutationFn: () => api('/cash-closing', {
      method: 'POST',
      body: { tanggal, modal_awal: Number(modalAwal) || 0, kas_fisik: Number(kasFisik) || 0, catatan },
    }),
    onSuccess: (r) => {
      setMsg(r.tutup_ulang
        ? `Penutupan ulang tersimpan. Selisih: ${rupiah(r.selisih)} (versi lama dicatat di audit).`
        : `Penutupan tersimpan. Selisih: ${rupiah(r.selisih)}`)
      qc.invalidateQueries()
    },
    onError: (e) => setMsg(e.message),
  })

  return (
    <>
      <h1>Tutup Kasir</h1>
      <div className="grid-2">
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Pratinjau kas seharusnya</h3>
          <input type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} style={{ width: '100%', marginBottom: 10 }} />
          {preview.data && (
            <div className="table-wrap"><table className="lb">
              <tbody>
                <tr><td>Modal awal</td><td>{rupiah(preview.data.rincian?.modal_awal)}</td></tr>
                <tr><td>Penjualan tunai</td><td>+ {rupiah(preview.data.rincian?.penjualan_tunai)}</td></tr>
                <tr><td>DP kasbon (dianggap tunai)</td><td>+ {rupiah(preview.data.rincian?.dp_kasbon)}</td></tr>
                <tr><td>Pelunasan utang pelanggan (tunai)</td><td>+ {rupiah(preview.data.rincian?.pelunasan_utang_tunai)}</td></tr>
                <tr><td>Pengeluaran tunai</td><td>− {rupiah(preview.data.rincian?.pengeluaran_tunai)}</td></tr>
                <tr><td>Pembelian tunai</td><td>− {rupiah(preview.data.rincian?.pembelian_tunai)}</td></tr>
                <tr><td>Bayar utang supplier (tunai)</td><td>− {rupiah(preview.data.rincian?.bayar_utang_supplier_tunai)}</td></tr>
                <tr><td><b>Kas sistem</b></td><td><b>{rupiah(kasSistem)}</b></td></tr>
              </tbody>
            </table></div>
          )}
          {preview.data?.sudah_tutup && (
            <p className="warning-text" style={{ marginTop: 8 }}>Tanggal ini sudah ditutup{isPemilik ? ' — kamu (pemilik) dapat menutup ulang.' : '.'}</p>
          )}
        </div>

        <div className="card">
          <h3 style={{ marginTop: 0 }}>Hitung fisik</h3>
          <div style={{ display: 'grid', gap: 8 }}>
            <label className="muted">Modal awal</label>
            <input type="number" value={modalAwal} onChange={(e) => setModalAwal(e.target.value)} />
            <label className="muted">Kas fisik hasil hitung</label>
            <input type="number" value={kasFisik} onChange={(e) => setKasFisik(e.target.value)} />
            {selisih !== null && (
              <p className={selisih === 0 ? 'success-text' : 'warning-text'}>
                Selisih = {rupiah(selisih)} {selisih !== 0 && '(boleh disimpan, dicatat)'}
              </p>
            )}
            <input placeholder="Catatan" value={catatan} onChange={(e) => setCatatan(e.target.value)} />
            <button className="btn-accent" disabled={kasFisik === '' || simpan.isPending} onClick={() => simpan.mutate()}>
              Simpan Penutupan
            </button>
          </div>
          {msg && <p className="muted" style={{ marginTop: 8 }}>{msg}</p>}
        </div>
      </div>

      <div className="card" style={{ marginTop: 14 }}>
        <h3 style={{ marginTop: 0 }}>Riwayat penutupan</h3>
        <Riwayat />
      </div>
    </>
  )
}

function Riwayat() {
  const { data } = useQuery({ queryKey: ['closings'], queryFn: () => api('/cash-closing') })
  return (
    <div className="table-wrap"><table className="lb">
      <thead><tr><th>Tanggal</th><th>Modal Awal</th><th>Kas Sistem</th><th>Kas Fisik</th><th>Selisih</th><th>Oleh</th></tr></thead>
      <tbody>
        {(data?.data || []).map((c) => (
          <tr key={c.id}>
            <td>{c.tanggal}</td>
            <td>{rupiah(c.modal_awal)}</td>
            <td>{rupiah(c.kas_sistem)}</td>
            <td>{rupiah(c.kas_fisik)}</td>
            <td className={c.selisih !== 0 ? 'warning-text' : 'success-text'}>{rupiah(c.selisih)}</td>
            <td>{c.user_nama}</td>
          </tr>
        ))}
      </tbody>
    </table></div>
  )
}
