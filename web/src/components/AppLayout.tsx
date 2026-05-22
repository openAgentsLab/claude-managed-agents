import { type ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { useAuth } from '@/App'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

interface NavItemProps {
  to: string
  children: ReactNode
}

function NavItem({ to, children }: NavItemProps) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors',
          isActive
            ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
            : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground',
        )
      }
    >
      {children}
    </NavLink>
  )
}

function NavSection({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-0.5">
      <p className="px-3 py-1 text-[11px] font-semibold uppercase tracking-wider text-sidebar-foreground/40">
        {label}
      </p>
      {children}
    </div>
  )
}

// ── Icons (inline SVG to avoid extra deps) ────────────────────────────────────

const IconChat = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path fillRule="evenodd" d="M2 5a2 2 0 012-2h8a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V5zm3 1h6v.5H5V6zm6 3H5v.5h6V9zm-2 3H5v.5h4v-.5z" clipRule="evenodd" />
    <path d="M15 7h1a2 2 0 012 2v5.5a1.5 1.5 0 01-3 0V7z" />
  </svg>
)

const IconPlug = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path d="M8.5 3.5a.5.5 0 00-1 0V5H6a1 1 0 000 2h.5v3A2.5 2.5 0 009 12.45V15.5a.5.5 0 001 0v-3.05A2.5 2.5 0 0012.5 10V7H13a1 1 0 100-2h-1.5V3.5a.5.5 0 00-1 0V5h-1.5V3.5z" />
  </svg>
)

const IconCode = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path fillRule="evenodd" d="M12.316 3.051a1 1 0 01.633 1.265l-4 12a1 1 0 11-1.898-.632l4-12a1 1 0 011.265-.633zM5.707 6.293a1 1 0 010 1.414L3.414 10l2.293 2.293a1 1 0 11-1.414 1.414l-3-3a1 1 0 010-1.414l3-3a1 1 0 011.414 0zm8.586 0a1 1 0 011.414 0l3 3a1 1 0 010 1.414l-3 3a1 1 0 11-1.414-1.414L16.586 10l-2.293-2.293a1 1 0 010-1.414z" clipRule="evenodd" />
  </svg>
)

const IconVault = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path fillRule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clipRule="evenodd" />
  </svg>
)

const IconSettings = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path fillRule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clipRule="evenodd" />
  </svg>
)

const IconModel = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path d="M13 7H7v6h6V7z" />
    <path fillRule="evenodd" d="M7 2a1 1 0 012 0v1h2V2a1 1 0 112 0v1h2a2 2 0 012 2v2h1a1 1 0 110 2h-1v2h1a1 1 0 110 2h-1v2a2 2 0 01-2 2h-2v1a1 1 0 11-2 0v-1H9v1a1 1 0 11-2 0v-1H5a2 2 0 01-2-2v-2H2a1 1 0 110-2h1V9H2a1 1 0 010-2h1V5a2 2 0 012-2h2V2zM5 5h10v10H5V5z" clipRule="evenodd" />
  </svg>
)

const IconUsers = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 17c.046-.327.07-.66.07-1a6.97 6.97 0 00-1.5-4.33A5 5 0 0119 16v1h-6.07zM6 11a5 5 0 015 5v1H1v-1a5 5 0 015-5z" />
  </svg>
)

const IconFolder = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
  </svg>
)

const IconServer = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path fillRule="evenodd" d="M2 5a2 2 0 012-2h12a2 2 0 012 2v2a2 2 0 01-2 2H4a2 2 0 01-2-2V5zm14 1a1 1 0 11-2 0 1 1 0 012 0zM2 13a2 2 0 012-2h12a2 2 0 012 2v2a2 2 0 01-2 2H4a2 2 0 01-2-2v-2zm14 1a1 1 0 11-2 0 1 1 0 012 0z" clipRule="evenodd" />
  </svg>
)

const IconDatabase = () => (
  <svg viewBox="0 0 20 20" fill="currentColor" className="size-4 shrink-0">
    <path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z" />
    <path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z" />
    <path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z" />
  </svg>
)

// ── Layout ────────────────────────────────────────────────────────────────────

interface AppLayoutProps {
  children: ReactNode
}

export default function AppLayout({ children }: AppLayoutProps) {
  const { username, tenantId, role, logout } = useAuth()
  const navigate = useNavigate()

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 flex flex-col border-r border-sidebar-border bg-sidebar">
        {/* Brand */}
        <div className="px-4 py-4 flex items-center gap-2">
          <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10 border border-primary/20">
            <svg viewBox="0 0 24 24" fill="none" className="size-4 text-primary" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
            </svg>
          </div>
          <span className="font-semibold text-sidebar-foreground text-sm">Forge</span>
        </div>

        <Separator className="bg-sidebar-border" />

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto px-2 py-3 space-y-4">
          <div className="space-y-0.5">
            <NavItem to="/">
              <IconChat />
              Sessions
            </NavItem>
          </div>

          <NavSection label="My Config">
            <NavItem to="/settings/projects">
              <IconFolder />
              Projects
            </NavItem>
            <NavItem to="/settings/memory">
              <IconDatabase />
              Memory Stores
            </NavItem>
            <NavItem to="/settings/vault">
              <IconVault />
              Vault
            </NavItem>
            <NavItem to="/settings/model">
              <IconModel />
              Model
            </NavItem>
            <NavItem to="/settings/agents">
              <IconModel />
              Agents
            </NavItem>
            <NavItem to="/settings/environments">
              <IconServer />
              Environments
            </NavItem>
            <NavItem to="/settings/mcp">
              <IconPlug />
              MCP Servers
            </NavItem>
            <NavItem to="/settings/skills">
              <IconCode />
              Skills
            </NavItem>
          </NavSection>

          {role === 'admin' && (
            <NavSection label="Admin">
              <NavItem to="/admin/tenant">
                <IconSettings />
                Tenant Settings
              </NavItem>
              <NavItem to="/admin/users">
                <IconUsers />
                Users
              </NavItem>
              <NavItem to="/admin/agents">
                <IconModel />
                Agents
              </NavItem>
              <NavItem to="/admin/environments">
                <IconServer />
                Tenant Envs
              </NavItem>
              <NavItem to="/admin/mcp">
                <IconPlug />
                Tenant MCP
              </NavItem>
              <NavItem to="/admin/skills">
                <IconCode />
                Tenant Skills
              </NavItem>
              <NavItem to="/admin/vault">
                <IconVault />
                Tenant Vault
              </NavItem>
            </NavSection>
          )}
        </nav>

        <Separator className="bg-sidebar-border" />

        {/* User footer */}
        <div className="px-3 py-3 space-y-1">
          <div className="text-xs text-sidebar-foreground/60 px-1 truncate">
            {username}{tenantId && <span className="text-sidebar-foreground/40"> · {tenantId}</span>}
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent/50 text-xs h-7"
            onClick={() => { logout(); navigate('/login') }}
          >
            Sign out
          </Button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}
