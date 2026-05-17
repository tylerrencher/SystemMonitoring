import { createContext, useContext, useState } from 'react'
import type { User } from '@/types/api'

export interface AuthContextValue {
  user: User | null
  login: (user: User) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  login: () => undefined,
  logout: () => undefined,
})

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(() => {
    try {
      const stored = localStorage.getItem('auth_user')
      return stored ? (JSON.parse(stored) as User) : null
    } catch {
      return null
    }
  })

  const login = (u: User) => {
    localStorage.setItem('auth_user', JSON.stringify(u))
    setUser(u)
  }

  const logout = () => {
    localStorage.removeItem('auth_user')
    setUser(null)
  }

  return <AuthContext.Provider value={{ user, login, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  return useContext(AuthContext)
}
