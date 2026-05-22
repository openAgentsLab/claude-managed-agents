import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { createSession, deleteSession } from '@/api/sessions'
import { useAuth } from '@/App'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'

interface StoredSession {
  id: string
  name: string
  createdAt: string
}

function loadSessions(): StoredSession[] {
  try {
    return JSON.parse(localStorage.getItem('forge_sessions') ?? '[]')
  } catch {
    return []
  }
}

function saveSessions(sessions: StoredSession[]) {
  localStorage.setItem('forge_sessions', JSON.stringify(sessions))
}

interface Props {
  activeSessionId: string
}

export default function SessionSidebar({ activeSessionId }: Props) {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [sessions, setSessions] = useState<StoredSession[]>(loadSessions)
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    function onStorage() { setSessions(loadSessions()) }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  useEffect(() => {
    saveSessions(sessions)
  }, [sessions])

  async function handleNew() {
    if (creating) return
    setCreating(true)
    try {
      const { session_id, project_id } = await createSession()
      const s: StoredSession = {
        id: session_id,
        name: `Session ${new Date().toLocaleString()}`,
        createdAt: new Date().toISOString(),
      }
      setSessions((prev) => {
        const next = [{ ...s, projectId: project_id }, ...prev]
        saveSessions(next)
        return next
      })
      navigate(`/chat/${session_id}`)
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    try { await deleteSession(id) } catch { /* best effort */ }
    setSessions((prev) => {
      const next = prev.filter((s) => s.id !== id)
      saveSessions(next)
      return next
    })
    if (id === activeSessionId) navigate('/')
  }

  return (
    <div className="w-60 flex-shrink-0 flex flex-col border-r border-border bg-sidebar">
      {/* Header */}
      <div className="px-4 py-4 flex items-center justify-between">
        <Button
          variant="ghost"
          className="font-semibold text-sidebar-foreground hover:text-sidebar-foreground px-2 h-auto"
          onClick={() => navigate('/')}
        >
          Forge
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={handleNew}
          disabled={creating}
          title="New session"
          className="w-7 h-7 text-sidebar-foreground/60 hover:text-sidebar-foreground"
        >
          <svg viewBox="0 0 16 16" fill="none" className="w-4 h-4">
            <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
        </Button>
      </div>

      <Separator className="bg-sidebar-border" />

      {/* Session list */}
      <ScrollArea className="flex-1 py-2">
        {sessions.length === 0 ? (
          <p className="px-4 py-3 text-sidebar-foreground/40 text-xs">No sessions</p>
        ) : (
          <div className="px-2 space-y-0.5">
            {sessions.map((s) => (
              <div
                key={s.id}
                onClick={() => navigate(`/chat/${s.id}`)}
                className={`group relative flex items-center px-3 py-2.5 rounded-lg cursor-pointer transition-colors ${
                  s.id === activeSessionId
                    ? 'bg-sidebar-accent text-sidebar-foreground'
                    : 'text-sidebar-foreground/60 hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'
                }`}
              >
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-medium truncate">{s.name}</p>
                </div>
                <button
                  onClick={(e) => handleDelete(s.id, e)}
                  className="opacity-0 group-hover:opacity-100 w-5 h-5 flex items-center justify-center text-sidebar-foreground/40 hover:text-destructive transition-colors ml-1 flex-shrink-0 rounded"
                >
                  ×
                </button>
              </div>
            ))}
          </div>
        )}
      </ScrollArea>

      {/* Footer */}
      <Separator className="bg-sidebar-border" />
      <div className="px-4 py-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={logout}
          className="w-full justify-start text-sidebar-foreground/40 hover:text-sidebar-foreground px-2 h-8 text-xs"
        >
          Sign out
        </Button>
      </div>
    </div>
  )
}
