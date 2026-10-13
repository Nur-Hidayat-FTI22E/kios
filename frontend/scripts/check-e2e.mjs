/* E2E semua fitur via UI nyata (Chrome headless) — pengisian input memakai
   native value setter + event 'input' agar React selalu menangkap nilai.
   Jalankan: node check-e2e.mjs — keluar 0 bila semua alur sukses. */
import puppeteer from 'puppeteer-core'

const BASE = 'http://localhost:5173'
const unik = Date.now().toString(36).slice(-4)
let gagal = 0
const catat = (nama, ok, detail = '') => {
  if (!ok) gagal++
  console.log(`${ok ? 'OK    ' : 'GAGAL '} ${nama}${detail ? ' :: ' + detail : ''}`)
  if (!ok) process.exitCode = 1
}

const browser = await puppeteer.launch({
  executablePath: '/usr/bin/google-chrome',
  headless: 'new',
  args: ['--no-sandbox', '--disable-dev-shm-usage'],
})
try {
  const page = await browser.newPage()
  const galatJaringan = []
  page.on('response', async (r) => {
    if (r.status() >= 400 && !r.url().includes('favicon')) {
      let bu = ''
      try { bu = (await r.text()).slice(0, 200) } catch { /* abaikan */ }
      galatJaringan.push(`${r.status()} ${r.url()} → ${bu}`)
    }
  })
  await page.setViewport({ width: 390, height: 844, isMobile: true })
  const tunggu = (ms) => new Promise(r => setTimeout(r, ms))
  const klik = async (teks, cakup = 'button') => {
    const el = await page.evaluateHandle((t, c) => [...document.querySelectorAll(c)].find(x => x.textContent.trim() === t || x.textContent.trim().startsWith(t)), teks, cakup)
    const t = await el.evaluate(e => { if (!e) return 'TIDAK_ADA'; e.click(); return null }).catch(() => 'hilang')
    if (t) throw new Error(`tombol "${teks}" tidak ada`)
    await tunggu(400)
  }
  // setIsi: set nilai lewat setter native + event input (React 18 menangkapnya)
  const setIsi = async (sel, val) => {
    const t = await page.evaluate((s, v) => {
      const el = document.querySelector(s)
      if (!el) return 'input tidak ada: ' + s
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
      setter.call(el, String(v))
      el.dispatchEvent(new Event('input', { bubbles: true }))
      return null
    }, sel, val)
    if (t) throw new Error(t)
    await tunggu(300)
  }
  const navigasi = async (rute) => {
    await page.goto(BASE + '/#' + rute, { waitUntil: 'networkidle2' })
    await page.reload({ waitUntil: 'networkidle2' })
    await tunggu(700)
  }
  const teksAda = (t) => page.evaluate((tt) => document.body.textContent.includes(tt), t)

  // A. LOGIN
  await page.goto(BASE + '/#/login', { waitUntil: 'networkidle2' })
  await setIsi('input:not([type="password"])', 'pemilik')
  await setIsi('input[type="password"]', 'admin123')
  await klik('Masuk')
  await page.waitForFunction(() => !location.hash.includes('login'), { timeout: 8000 })
  catat('FR-AUTH-01 Login pemilik', true)

  // S. TAMBAH PRODUK BARU (kasus utama user)
  const namaProduk = `Teh Kotak ${unik}`
  await navigasi('/produk')
  await klik('+ Produk Baru')
  for (const [ph, val] of [['Nama', namaProduk], ['Satuan (pcs)', 'pcs'], ['Harga beli', '2500'], ['Harga jual', '4000'], ['Stok awal', '50'], ['Stok minimum', '5']]) {
    await setIsi(`input[placeholder="${ph}"]`, val)
  }
  await klik('Simpan')
  await tunggu(800)
  catat('FR-PRD-01 Tambah produk baru (muncul di daftar)', await teksAda(namaProduk))

  // K1. KASIR: cari produk BARU → pilih → tunai → Bayar (kasus user)
  const namaPendek = 'Teh Kotak'
  await navigasi('/kasir')
  await setIsi('.search-big', namaPendek)
  await tunggu(900)
  const adaDiHasil = await page.evaluate((n) => [...document.querySelectorAll('button')].some(b => b.textContent.includes(n)), namaProduk)
  catat('FR-POS-01 Produk baru terbaca di pencarian Kasir', adaDiHasil, adaDiHasil ? '' : 'produk baru tidak muncul di hasil pencarian')
  await page.evaluate((n) => [...document.querySelectorAll('button')].find(b => b.textContent.includes(n))?.click(), namaProduk)
  await tunggu(500)
  await klik('Uang pas')
  await klik('Bayar')
  await tunggu(900)
  catat('FR-POS-04/05 Jual tunai sukses (produk baru)', await teksAda('tersimpan'))

  // K2. KASIR: kasbon dengan pelanggan cepat (FR-DBT-06)
  await setIsi('.search-big', 'bumbu')
  await tunggu(800)
  await page.evaluate(() => [...document.querySelectorAll('button')].find(b => b.textContent.includes('bumbu'))?.click())
  await tunggu(500)
  await klik('Kasbon')
  await setIsi('input[placeholder="Nama pelanggan baru"]', `Bu Sari ${unik}`)
  await klik('Tambah')
  await tunggu(600)
  await setIsi('input[placeholder="DP tunai (boleh 0)"]', '2000')
  await klik('Bayar')
  await tunggu(900)
  catat('FR-POS-04 Jual kasbon + pelanggan cepat', await teksAda('kembalian') || await teksAda('tersimpan'))

  // H. UTANG: bayar utang pelanggan sampai lunas
  await navigasi('/utang')
  const punyaBayar = await page.evaluate(() => [...document.querySelectorAll('tbody button')].some(b => b.textContent === 'Bayar'))
  if (punyaBayar) {
    await page.evaluate(() => [...document.querySelectorAll('tbody button')].find(b => b.textContent === 'Bayar').click())
    await tunggu(600)
    await klik('Simpan')
    await tunggu(900)
    catat('FR-DBT-02 Bayar utang pelanggan (lunas)', await teksAda('Utang lunas'))
  } else {
    catat('FR-DBT-02 Bayar utang pelanggan', false, 'tombol Bayar tidak ditemukan — utang kasbon tidak terbentuk?')
  }

  // B. PEMBELIAN / BARANG MASUK
  await navigasi('/pembelian')
  await setIsi('input[placeholder="Cari produk…"]', namaPendek)
  await tunggu(800)
  await page.evaluate((n) => [...document.querySelectorAll('button')].find(b => b.textContent.includes(n))?.click(), namaProduk)
  await tunggu(600)
  // bayar penuh agar tidak jatuh ke utang supplier (validasi FR-PUR-03 sudah benar di server)
  await setIsi('input[placeholder="Dibayar"]', '999999')
  await tunggu(400)
  await klik('Simpan Pembelian')
  await tunggu(1000)
  catat('FR-PUR-01 Catat pembelian (barang masuk)', await teksAda('Pembelian #'))

  // P. PENGELUARAN
  const hari = new Date(); hari.setMinutes(hari.getMinutes() - hari.getTimezoneOffset())
  const tgl = hari.toISOString().slice(0, 10)
  await navigasi('/pengeluaran')
  await setIsi('input[type="date"]', tgl)
  await page.evaluate(() => { const s = document.querySelector('select'); if (s && s.options.length > 1) { s.selectedIndex = 1; s.dispatchEvent(new Event('change', { bubbles: true })) } })
  await setIsi('input[type="number"]', '8000')
  await klik('Simpan')
  await tunggu(900)
  catat('FR-EXP-01 Catat pengeluaran', await teksAda('dicatat'))

  // T. TUTUP KASIR
  await navigasi('/tutup-kasir')
  await tunggu(900)
  const kasSistem = await page.evaluate(() => {
    const td = [...document.querySelectorAll('td')].find(x => x.textContent.trim() === 'Kas sistem')
    return td ? td.nextElementSibling.textContent.replace(/\D/g, '') : ''
  })
  if (kasSistem === '') {
    catat('FR-CLS-01 Pratinjau kas sistem', false, 'baris "Kas sistem" tidak ada')
  } else {
    catat('FR-CLS-01 Pratinjau kas sistem', true, 'kas sistem = ' + kasSistem)
    // input kas fisik = input number kedua (pertama modal awal)
    await page.evaluate((v) => {
      const angka = [...document.querySelectorAll('input[type="number"]')]
      const el = angka[1] || angka[0]
      el.focus()
    })
    await setIsi('input[placeholder="Catatan"]', 'E2E')
    // isi kas fisik dengan jumlah kas sistem
    await page.evaluate((v) => {
      const angka = [...document.querySelectorAll('input[type="number"]')]
      const el = angka[1] || angka[0]
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
      setter.call(el, String(v))
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }, kasSistem)
    await tunggu(400)
    await klik('Simpan Penutupan')
    await tunggu(1000)
    catat('FR-CLS-02/03 Tutup kasir', await teksAda('Penutupan tersimpan') || await teksAda('Penutupan ulang tersimpan'))
  }

  // L. LAPORAN: omzet terisi
  await navigasi('/laporan')
  await tunggu(1100)
  const omzet = await page.evaluate(() => {
    const c = [...document.querySelectorAll('.stat-cards .card')].find(x => x.textContent.includes('Omzet'))
    return c ? c.querySelector('.val')?.textContent : ''
  })
  catat('FR-RPT-01/02 Laporan omzet terisi', !!omzet && omzet.trim() !== 'Rp 0', 'omzet = ' + omzet)

  // E. EKSPOR EXCEL (dengan parameter tanggal — kontrak API)
  const eksporOk = await page.evaluate(async () => {
    const q = new URLSearchParams({ dari: '2026-01-01', sampai: '2026-12-31' })
    const tk = localStorage.getItem('token')
    const r = await fetch('/api/v1/reports/export?' + q, { headers: { Authorization: 'Bearer ' + tk } })
    if (!r.ok) return false
    const b = await r.blob()
    const head = r.headers.get('Content-Disposition') || ''
    return /laporan-kios_/.test(head) && b.size > 4000 && /spreadsheet/.test(r.headers.get('Content-Type') || '')
  })
  catat('FR-RPT-05 Ekspor Excel valid', eksporOk)

  // KONSISTENSI STOK (FR-STK-05) + saldo utang
  const konsisten = await page.evaluate(async () => {
    const tk = localStorage.getItem('token')
    const r = await fetch('/api/v1/stock/consistency', { headers: { Authorization: 'Bearer ' + tk } })
    const j = await r.json()
    return (j.data || []).length === 0
  })
  catat('FR-STK-05 Konsistensi stok seluruh pergerakan', konsisten)

  const saldoUtang = await page.evaluate(async () => {
    const tk = localStorage.getItem('token')
    const r = await fetch('/api/v1/debts?tipe=pelanggan&status=lunas', { headers: { Authorization: 'Bearer ' + tk } })
    const j = await r.json()
    return (j.data || []).every(d => d.lunas)
  })
  catat('FR-DBT-02 Utang lunas terekam', saldoUtang)

  console.log('\n== respons HTTP >= 400 selama E2E ==')
  console.log(galatJaringan.length ? galatJaringan.join('\n') : 'tidak ada')
  console.log(gagal === 0 ? '\nSEMUA ALUR E2E SUKSES' : `\n${gagal} alur gagal`)
} finally { await browser.close() }
