import puppeteer from 'puppeteer-core'

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: 'new', args: ['--no-sandbox'] })
const fails = []
const HAL = ['/', '/kasir', '/produk', '/laporan', '/utang']

// login sekali (localStorage dibagi antar tab)
{
  const p = await browser.newPage()
  await p.setViewport({ width: 360, height: 780 })
  await p.goto('http://localhost:5173/#/login', { waitUntil: 'networkidle2' })
  await p.type('input[placeholder="Username"]', 'pemilik')
  await p.type('input[placeholder="Password"]', 'admin123')
  await p.click('button.btn-accent')
  await p.waitForFunction(() => !location.hash.includes('login'), { timeout: 10000 })
  // klik toggle tema di topbar
  const btn = await p.waitForFunction(() => [...document.querySelectorAll('.topbar button')].find(b => b.textContent.includes('Gelap')))
  await btn.click()
  await p.waitForFunction(() => document.documentElement.dataset.theme === 'dark', { timeout: 5000 })
  console.log('toggle 🌙 Gelap diklik → data-theme=dark ✓')
  // persistensi: muat ulang harus tetap gelap
  await p.reload({ waitUntil: 'networkidle2' })
  await p.waitForFunction(() => document.documentElement.dataset.theme === 'dark', { timeout: 5000 })
  const btn2 = await p.waitForFunction(() => [...document.querySelectorAll('.topbar button')].find(b => b.textContent.includes('Terang')))
  if (!btn2) fails.push('tombol tidak berubah jadi "Terang" setelah reload')
  console.log('setelah reload tetap gelap & tombol jadi "☀️ Terang" ✓')
  await p.close()
}

const VP = [['hp-360', 360, 780], ['tablet-768', 768, 1024], ['desktop-1366', 1366, 900]]
for (const [nama, w, h] of VP) {
  const page = await browser.newPage()
  await page.setViewport({ width: w, height: h })
  for (const hal of HAL) {
    await page.goto('http://localhost:5173/#' + hal, { waitUntil: 'networkidle2' })
    await new Promise(r => setTimeout(r, 400))
    const cek = await page.evaluate(() => {
      const bg = getComputedStyle(document.body).backgroundColor
      const teks = getComputedStyle(document.body).color
      const katas = [...document.querySelectorAll('.card')].slice(0, 3)
        .map(c => ({ bg: getComputedStyle(c).backgroundColor, teks: getComputedStyle(c).color }))
      const nav = document.querySelector(window.innerWidth < 900 ? '.tabbar' : '.desknav')
      const navStyle = nav ? { disp: getComputedStyle(nav).display, bg: getComputedStyle(nav).backgroundColor } : null
      const aktif = [...document.querySelectorAll(window.innerWidth < 900 ? '.tabbar a.active' : '.desknav a.active')][0]
      return {
        overX: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) - window.innerWidth,
        bg, teks, katas, navStyle,
        aktifColor: aktif ? getComputedStyle(aktif).color : null,
        darkToken: document.documentElement.dataset.theme,
      }
    })
    // token gelap: --bg #23262d = rgb(35,38,45)
    if (cek.darkToken !== 'dark') fails.push(`[${nama} ${hal}] data-theme=${cek.darkToken}`)
    if (cek.bg !== 'rgb(35, 38, 45)') fails.push(`[${nama} ${hal}] body bg=${cek.bg}, bukan token gelap`)
    if (cek.teks !== 'rgb(243, 244, 246)') fails.push(`[${nama} ${hal}] teks body=${cek.teks}, bukan token gelap`)
    for (const k of cek.katas) {
      if (k.bg !== 'rgb(35, 38, 45)' || k.bg === k.teks) fails.push(`[${nama} ${hal}] kartu tidak kontras: bg=${k.bg} teks=${k.teks}`)
    }
    if (cek.navStyle) {
      if (cek.navStyle.bg !== 'rgb(35, 38, 45)') fails.push(`[${nama} ${hal}] nav bg=${cek.navStyle.bg} (masih terang?)`)
      if (cek.navStyle.disp !== (w < 900 ? 'flex' : 'flex')) fails.push(`[${nama} ${hal}] nav display=${cek.navStyle.disp}`)
    } else fails.push(`[${nama} ${hal}] nav tidak ketemu`)
    if (cek.aktifColor && cek.aktifColor !== 'rgb(96, 165, 250)') fails.push(`[${nama} ${hal}] warna nav aktif=${cek.aktifColor}, bukan var(--accent) gelap`)
    if (cek.overX > 1) fails.push(`[${nama} ${hal}] overflow-x=${cek.overX}px`)
    console.log(`${nama} ${hal}: bg=${cek.bg} nav=${cek.navStyle?.disp} aktif=${cek.aktifColor} overX=${cek.overX} ✓`)
  }
  await page.goto('http://localhost:5173/#/kasir', { waitUntil: 'networkidle2' })
  await new Promise(r => setTimeout(r, 500))
  await page.screenshot({ path: `/tmp/tema-gelap-${nama}.png` })
  console.log(`screenshot /tmp/tema-gelap-${nama}.png tersimpan`)
  await page.close()
}
await browser.close()
if (fails.length) { console.log('\nGAGAL:\n' + fails.join('\n')); process.exit(1) }
console.log('\nTEMA GELAP SEMUA UKURAN OK')
