import puppeteer from 'puppeteer-core'
import fs from 'fs'
import { execSync } from 'child_process'

const CK = fs.existsSync('/tmp/dl') || (fs.mkdirSync('/tmp/dl', { recursive: true }), true)
const browser = await puppeteer.launch({
  executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome',
  headless: 'new',
  args: ['--no-sandbox', '--window-size=390,844'],
})
const page = await browser.newPage()
await page.setViewport({ width: 390, height: 844 })
const fails = []
page.on('response', (r) => { if (r.status() >= 400 && !r.url().includes('favicon')) fails.push(r.status() + ' ' + r.url()) })

// CDP: izinkan unduhan ke /tmp/dl
const cdp = await page.createCDPSession()
await cdp.send('Browser.setDownloadBehavior', { behavior: 'allow', downloadPath: '/tmp/dl' })

await page.goto('http://localhost:5173/#/login', { waitUntil: 'networkidle2' })
await page.type('input[placeholder="Username"]', 'pemilik')
await page.type('input[placeholder="Password"]', 'admin123')
await page.click('button.btn-accent')
await page.waitForFunction(() => !location.hash.includes('login'), { timeout: 10000 })
console.log('login OK, toklen', (await page.evaluate(() => localStorage.getItem('token') || '')).length)

await page.goto('http://localhost:5173/#/laporan', { waitUntil: 'networkidle2' })
await page.waitForFunction(() => document.body.innerText.includes('Omzet'), { timeout: 10000 })
console.log('halaman laporan OK')

const btn = await page.waitForFunction(() => [...document.querySelectorAll('button')].find(b => b.textContent.includes('Ekspor Excel')))
await btn.click()
await page.waitForFunction(() => document.body.innerText.includes('berhasil diunduh'), { timeout: 20000 })
console.log('pesan sukses tampil:', await page.evaluate(() => [...document.querySelectorAll('p')].map(p => p.textContent).find(t => t.includes('diunduh'))))

await new Promise(r => setTimeout(r, 1000))
const files = fs.readdirSync('/tmp/dl').filter(f => f.endsWith('.xlsx'))
if (!files.length) { console.log('GAGAL: tidak ada file di /tmp/dl'); process.exit(1) }
const f = '/tmp/dl/' + files[files.length - 1]
const head = fs.readFileSync(f).slice(0, 2).toString()
if (head !== 'PK') { console.log('GAGAL: bukan file xlsx, head=' + head); process.exit(1) }
// tampilkan daftar sheet dalam file
const sheets = execSync(`unzip -l "${f}" | grep -c "xl/worksheets/" || true`).toString().trim()
console.log('file:', files[files.length - 1], '| ukuran:', fs.statSync(f).size, 'byte | jumlah sheet:', sheets)

if (fails.length) { console.log('HTTP >=400:', fails.join('; ')); process.exit(1) }
console.log('SEMUA OK')
await browser.close()
