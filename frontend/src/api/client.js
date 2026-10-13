const BASE = '/api/v1'

export function getToken() {
  return localStorage.getItem('token') || ''
}

export function setToken(t) {
  if (t) localStorage.setItem('token', t)
  else localStorage.removeItem('token')
}

function newIdemKey() {
  return 'idem-' + Date.now() + '-' + Math.random().toString(36).slice(2, 10)
}

export async function api(path, { method = 'GET', body, headers = {}, idempotent = false } = {}) {
  const h = { 'Content-Type': 'application/json', ...headers }
  const tk = getToken()
  if (tk) h['Authorization'] = 'Bearer ' + tk
  if (idempotent && method === 'POST') h['Idempotency-Key'] = newIdemKey()
  const resp = await fetch(BASE + path, {
    method,
    headers: h,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await resp.text()
  let data
  try { data = text ? JSON.parse(text) : {} } catch { data = {} }
  if (!resp.ok) {
    if (resp.status === 401) {
      setToken('')
      window.location.hash = '#/login'
    }
    const msg = data?.error?.pesan || 'Terjadi kesalahan (' + resp.status + ')'
    throw new Error(msg)
  }
  return data
}

export function rupiah(n) {
  // format Rp 12.500 (NFR-06)
  return 'Rp ' + Number(n || 0).toLocaleString('id-ID')
}

export function tanggalRingkas(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  const bulan = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
  return d.getDate() + ' ' + bulan[d.getMonth()] + ' ' + d.getFullYear()
}
