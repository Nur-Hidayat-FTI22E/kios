import puppeteer from 'puppeteer-core'

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: 'new', args: ['--no-sandbox'] })
const fails = []

// login sekali (localStorage dibagi antar tab)
{
  const p = await browser.newPage()
  await p.setViewport({ width: 1200, height: 900 })
  await p.goto('http://localhost:5173/#/login', { waitUntil: 'networkidle2' })
  await p.type('input[placeholder="Username"]', 'pemilik')
  await p.type('input[placeholder="Password"]', 'admin123')
  await p.click('button.btn-accent')
  await p.waitForFunction(() => !location.hash.includes('login'), { timeout: 10000 })
  await p.close()
}

for (const tema of ['terang', 'gelap']) {
  const page = await browser.newPage()
  await page.setViewport({ width: 1200, height: 900 })
  if (tema === 'gelap') await page.evaluateOnNewDocument(() => localStorage.setItem('tema', 'gelap'))
  await page.goto('http://localhost:5173/#/laporan', { waitUntil: 'networkidle2' })
  await page.waitForFunction(() => document.body.innerText.includes('Tren harian') && document.body.innerText.includes('Terlaris'), { timeout: 15000 })
  await new Promise(r => setTimeout(r, 2500))

  const cek = await page.evaluate(() => {
    const n = (s) => (s || '').replace(/\s/g, '')
    const q = (sel, prop) => { const e = document.querySelector(sel); return e ? n(getComputedStyle(e)[prop]) : null }
    return {
      axis: q('.recharts-cartesian-axis-line', 'stroke'),
      tick: q('.recharts-cartesian-axis-tick-value', 'fill'),
      curve: q('.recharts-line-curve', 'stroke'),
      dot: q('.recharts-line-dot', 'fill'),
      bar: q('.recharts-bar-rectangle path, .recharts-bar-rectangle', 'fill'),
      border: q('table.lb th', 'borderBottomColor'),
      nav: q('.desknav', 'boxShadow'),
      // aturan tooltip ikut tema harus ada di stylesheet
      tooltipRule: [...document.styleSheets].some(ss => {
        try { return [...ss.cssRules].some(r => r.selectorText === '.recharts-default-tooltip' && r.style.background.includes('var(--bg)')) }
        catch { return false }
      }),
      // sisa hardcode #666 di atribut sumbu?
      axisAttr: (document.querySelector('.recharts-cartesian-axis-line')?.getAttribute('stroke') || ''),
    }
  })

  // token: terang accent rgb(37,99,235), success rgb(21,128,61), muted rgb(75,85,99),
  //        line rgba(0,0,0,0.06), shadow-dark rgb(195,200,212)
  //        gelap  accent rgb(96,165,250), success rgb(74,222,128), muted rgb(209,213,219),
  //        line rgba(255,255,255,0.09), shadow-dark rgb(20,22,26)
  const H = tema === 'gelap'
    ? { acc: 'rgb(96,165,250)', suk: 'rgb(74,222,128)', mut: 'rgb(209,213,219)', line: 'rgba(255,255,255,0.09)', sd: 'rgb(20,22,26)' }
    : { acc: 'rgb(37,99,235)', suk: 'rgb(21,128,61)', mut: 'rgb(75,85,99)', line: 'rgba(0,0,0,0.06)', sd: 'rgb(195,200,212)' }
  const mulai = (dapat, harus) => dapat && dapat.startsWith(harus.slice(0, harus.indexOf(')') + 1))

  if (!mulai(cek.axis, H.mut)) fails.push(`[${tema}] garis sumbu=${cek.axis} harap ${H.mut}`)
  if (!mulai(cek.tick, H.mut)) fails.push(`[${tema}] teks sumbu=${cek.tick} harap ${H.mut}`)
  if (cek.curve && !mulai(cek.curve, H.acc)) fails.push(`[${tema}] kurva omzet=${cek.curve} harap ${H.acc}`)
  if (cek.dot && !mulai(cek.dot, H.acc)) fails.push(`[${tema}] dot tren=${cek.dot} harap ${H.acc}`)
  if (!mulai(cek.bar, H.suk)) fails.push(`[${tema}] bar laba=${cek.bar} harap ${H.suk}`)
  if (!mulai(cek.border, H.line)) fails.push(`[${tema}] garis tabel=${cek.border} harap ${H.line}`)
  if (!cek.nav || !cek.nav.includes(H.sd)) fails.push(`[${tema}] bayangan nav=${cek.nav} harap memuat ${H.sd}`)
  if (!cek.tooltipRule) fails.push(`[${tema}] aturan CSS .recharts-default-tooltip (background var(--bg)) tidak ketemu`)
  // Atribut #666 mahupun价值 diinjeksi Recharts saat render (bukan kode aplikasi) — yang penting
  // nilai terkomputasi mengikuti tema (sudah diuji via 'axis' & 'tick' di atas). Catat saja.
  if (cek.axisAttr === '#666' || cek.axisAttr === '#666666') console.log(`  (info) atribut bawaan Recharts ${cek.axisAttr} tetap ada & tertimpa CSS tema — wajar`)
  console.log(`${tema}: axis=${cek.axis} tick=${cek.tick} curve=${cek.curve} dot=${cek.dot} bar=${cek.bar} border=${cek.border} navShadow=${(cek.nav || '').slice(0, 40)} tooltipRule=${cek.tooltipRule} axisAttr=${cek.axisAttr}`)
  await page.screenshot({ path: `/tmp/warna-${tema}.png` })
  await page.close()
}
await browser.close()
if (fails.length) { console.log('\nGAGAL:\n' + fails.join('\n')); process.exit(1) }
console.log('\nWARNA TEMA OK (terang & gelap)')
