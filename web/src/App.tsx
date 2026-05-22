import { createContext, useContext, useState, type ReactNode } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import LoginPage from '@/pages/LoginPage'
import SessionsPage from '@/pages/SessionsPage'
import AdminTenantPage from '@/pages/AdminTenantPage'
import AdminUsersPage from '@/pages/AdminUsersPage'
import AdminMCPPage from '@/pages/AdminMCPPage'
import AdminSkillsPage from '@/pages/AdminSkillsPage'
import AdminVaultPage from '@/pages/AdminVaultPage'
import AdminEnvironmentsPage from '@/pages/AdminEnvironmentsPage'
import AdminAgentsPage from '@/pages/AdminAgentsPage'
import UserMCPPage from '@/pages/UserMCPPage'
import UserSkillsPage from '@/pages/UserSkillsPage'
import UserModelPage from '@/pages/UserModelPage'
import UserAgentsPage from '@/pages/UserAgentsPage'
import VaultPage from '@/pages/VaultPage'
import ProjectsPage from '@/pages/ProjectsPage'
import EnvironmentsPage from '@/pages/EnvironmentsPage'
import MemoryStoresPage from '@/pages/MemoryStoresPage'
import { logout as apiLogout } from '@/api/auth'
import { decodeToken, usernameFromSub } from '@/lib/jwt'

// ── Auth context ──────────────────────────────────────────────────────────────

interface AuthContextValue {
  token: string | null
  username: string
  tenantId: string
  role: string
  login: (token: string) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

function parseToken(t: string) {
  try {
    const p = decodeToken(t)
    return { username: usernameFromSub(p.sub), tenantId: p.tid, role: p.role }
  } catch {
    return { username: '', tenantId: '', role: '' }
  }
}

const emptyInfo = { username: '', tenantId: '', role: '' }

function AuthProvider({ children }: { children: ReactNode }) {
  const storedToken = localStorage.getItem('forge_token')
  const [token, setToken] = useState<string | null>(storedToken)
  const [userInfo, setUserInfo] = useState(storedToken ? parseToken(storedToken) : emptyInfo)

  function login(t: string) {
    localStorage.setItem('forge_token', t)
    setToken(t)
    setUserInfo(parseToken(t))
  }

  async function logout() {
    try { await apiLogout() } catch { /* best-effort */ }
    localStorage.removeItem('forge_token')
    setToken(null)
    setUserInfo(emptyInfo)
  }

  return (
    <AuthContext.Provider value={{ token, ...userInfo, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

// ── Route guards ──────────────────────────────────────────────────────────────

function PrivateRoute({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AdminRoute({ children }: { children: ReactNode }) {
  const { token, role } = useAuth()
  if (!token) return <Navigate to="/login" replace />
  if (role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}

// ── App ───────────────────────────────────────────────────────────────────────

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />

          {/* Main */}
          <Route path="/" element={<PrivateRoute><SessionsPage /></PrivateRoute>} />
          <Route path="/chat/:sessionId" element={<PrivateRoute><SessionsPage /></PrivateRoute>} />

          {/* User config */}
          <Route path="/settings/projects" element={<PrivateRoute><ProjectsPage /></PrivateRoute>} />
          <Route path="/settings/environments" element={<PrivateRoute><EnvironmentsPage /></PrivateRoute>} />
          <Route path="/settings/memory" element={<PrivateRoute><MemoryStoresPage /></PrivateRoute>} />
          <Route path="/settings/mcp" element={<PrivateRoute><UserMCPPage /></PrivateRoute>} />
          <Route path="/settings/skills" element={<PrivateRoute><UserSkillsPage /></PrivateRoute>} />
          <Route path="/settings/vault" element={<PrivateRoute><VaultPage /></PrivateRoute>} />
          <Route path="/settings/model" element={<PrivateRoute><UserModelPage /></PrivateRoute>} />
          <Route path="/settings/agents" element={<PrivateRoute><UserAgentsPage /></PrivateRoute>} />

          {/* Admin */}
          <Route path="/admin/tenant" element={<AdminRoute><AdminTenantPage /></AdminRoute>} />
          <Route path="/admin/users" element={<AdminRoute><AdminUsersPage /></AdminRoute>} />
          <Route path="/admin/environments" element={<AdminRoute><AdminEnvironmentsPage /></AdminRoute>} />
          <Route path="/admin/mcp" element={<AdminRoute><AdminMCPPage /></AdminRoute>} />
          <Route path="/admin/skills" element={<AdminRoute><AdminSkillsPage /></AdminRoute>} />
          <Route path="/admin/vault" element={<AdminRoute><AdminVaultPage /></AdminRoute>} />
          <Route path="/admin/agents" element={<AdminRoute><AdminAgentsPage /></AdminRoute>} />

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
