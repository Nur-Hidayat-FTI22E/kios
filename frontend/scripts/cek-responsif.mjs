import puppeteer from 'puppeteer-core'

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: 'new', args: ['--no-sandbox'] })
const vpList = [
  ['HP kecil', 360, 780],
  ['HP biasa', 414, 896],
  ['Tablet', 768, 1024],
  ['Laptop', 1366, 900],
]
const HAL = ['/', '/produk', '/stok', '/kasir', '/utang', '/pengeluaran', '/tutup-kasir', '/laporan', '/pengguna', '/pembelian']
let fails = []

// login satu kali sekadar menyiapkan sesi; tab baru memakai localStorage yang sama
{
  const page = await browser.newPage()
  await page.setViewport({ width: 360, height: 780 })
  await page.goto('http://localhost:5173/#/login', { waitUntil: 'networkidle2' })
  await page.type('input[placeholder="Username"]', 'pemilik')
  await page.type('input[placeholder="Password"]', 'admin123')
  await page.click('button.btn-accent')
  await page.waitForFunction(() => !location.hash.includes('login'), { timeout: 10000 })
  await page.close()
}

for (const [nama, w, h] of vpList) {
  const page = await browser.newPage()
  await page.setViewport({ width: w, height: h })
  for (const hal of HAL) {
    await page.goto('http://localhost:5173/#' + hal, { waitUntil: 'networkidle2' })
    await new Promise(r => setTimeout(r, 350))
    const cek = await page.evaluate(() => {
      const doc = document.documentElement
      const tabbar = document.querySelector('.tabbar')
      const desknav = document.querySelector('.desknav')
      const tab = tabbar?.getBoundingClientRect()
      const top = document.querySelector('.topbar')?.getBoundingClientRect()
      return {
        overX: doc.scrollWidth - window.innerWidth,
        bodyOver: document.body.scrollWidth - window.innerWidth,
        tabbar: tabbar ? getComputedStyle(tabbar).display : 'tidak ada',
        desknav: desknav ? getComputedStyle(desknav).display : 'tidak ada',
        topH: top ? top.height : -1,
        tabBottom: tab ? tab.bottom : -1,
        innerH: window.innerHeight,
      }
    })
    if (cek.overX > 1 || cek.bodyOver > 1) fails.push(`[${nama} ${hal}] overflow-x: doc+${cek.overX}px body+${cek.bodyOver}px`)
    if (w < 900 && cek.tabbar !== 'flex') fails.push(`[${nama} ${hal}] tabbar tidak tampil di HP (display=${cek.tabbar})`)
    if (w >= 900 && (cek.desknav !== 'flex' || cek.tabbar !== 'none')) fails.push(`[${nama} ${hal}] desktop: desknav=${cek.desknav} tabbar=${cek.tabbar}`)
    if (w < 900 && cek.topH > 90) fails.push(`[${nama} ${hal}] topbar terlalu tinggi: ${cek.topH}px`)
    if (w < 900 && cek.tabBottom < cek.innerH - 2) fails.push(`[${nama} ${hal}] tabbar tidak menempel bawah: ${cek.tabBottom} < ${cek.innerH}`)
    console.log(`${nama} (${w}px) ${hal}: overX=${cek.overX} tabbar=${cek.tabbar} desknav=${cek.desknav} topbarH=${Math.round(cek.topH)}px ✓`)
  }
  // klik tab bawah di HP: pastikan navigasi tetap berfungsi
  if (w === 360) {
    await page.goto('http://localhost:5173/#/', { waitUntil: 'networkidle2' })
    for (const l of ['Kasir', 'Utang', 'Tutup Kasir']) {
      const ok = await page.evaluate((lbl) => {
        const a = [...document.querySelectorAll('.tabbar a')].find(a => a.textContent.includes(lbl))
        if (!a) return false
        a.scrollIntoView({ block: 'nearest', inline: 'center', behavior: 'instant' })
        a.click()
        return true
      }, l)
      if (!ok) { fails.push(`[tab] tab "${l}" tidak ditemukan`); continue }
      try {
        await page.waitForFunction((lbl) => [...document.querySelectorAll('.tabbar a')].some(a => a.classList.contains('active') && a.textContent.includes(lbl)), { timeout: 4000 }, l)
      } catch { fails.push(`[tab] tab "${l}" tidak jadi aktif setelah diklik`) }
    }
    console.log('klik tab bawah ✓ (Kasir, Utang, Tutup Kasir aktiv)')
  }
  await page.close()
}

// screenshot untuk pengecekan visual
const page = await browser.newPage()
for (const [nama, w, h] of [['hp-360', 360, 780], ['desktop-1366', 1366, 900]]) {
  await page.setViewport({ width: w, height: h })
  await page.goto('http://localhost:5173/#/kasir', { waitUntil: 'networkidle2' })
  await new Promise(r => setTimeout(r, 600))
  await page.screenshot({ path: `/tmp/vp-${nama}.png` })
  console.log(`screenshot /tmp/vp-${nama}.png tersimpan`)
}
await page.close()
await browser.close()

if (fails.length) { console.log('\nGAGAL:\n' + fails.join('\n')); process.exit(1) }
console.log('\nSEMUA VIEWPORT OK')
