import { createContext, useContext, useEffect, useState } from 'react'
import { api, getToken, setToken } from './api/client'

const AuthCtx = createContext({ user: null })

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(!!getToken())

  useEffect(() => {
    if (!getToken()) return
    api('/auth/me')
      .then(setUser)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const login = async (username, password) => {
    const r = await api('/auth/login', { method: 'POST', body: { username, password } })
    setToken(r.token)
    setUser(r.user)
    return r.user
  }
  const logout = () => {
    api('/auth/logout', { method: 'POST' }).catch(() => {})
    setToken('')
    setUser(null)
  }

  return <AuthCtx.Provider value={{ user, loading, login, logout }}>{children}</AuthCtx.Provider>
}

export function useAuth() {
  return useContext(AuthCtx)
}

export function useIsPemilik() {
  const { user } = useAuth()
  return user?.peran === 'pemilik'
}
