import { useState, useEffect, useRef, type ReactNode } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { listSessions, createSession, deleteSession, getSession, getSessionHistory, updateSessionTitle, clearHistory, type SessionListItem, type HarnessEvent } from '@/api/sessions'
import { streamRun, type SSEEvent } from '@/api/chat'
import { listProjects, type Project } from '@/api/projects'
import { listMemoryStores, type MemoryStore } from '@/api/memory'
import { listAgents, type AgentResponse } from '@/api/agents'
import {
  listResources, addFileResource, addFileUrlResource, addGitResource, removeResource,
  listOutputs, downloadOutput,
  type ResourceItem, type OutputEntry,
} from '@/api/resources'
import MessageList, { type Message, type MessageSegment } from '@/components/MessageList'
import MessageInput from '@/components/MessageInput'
import AppLayout from '@/components/AppLayout'
import { useAuth } from '@/App'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

// ── New Session Modal ─────────────────────────────────────────────────────────

function NewSessionModal({
  onClose,
  onCreate,
  creating,
}: {
  onClose: () => void
  onCreate: (projectId?: string, memoryStores?: string[], agentId?: string) => void
  creating: boolean
}) {
  const [agents, setAgents] = useState<AgentResponse[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [memoryStores, setMemoryStores] = useState<MemoryStore[]>([])
  const [loading, setLoading] = useState(true)

  const defaultAgent = agents.find((a) => a.is_default)
  const [agentId, setAgentId] = useState('')
  const [projectId, setProjectId] = useState('')
  const [selectedStores, setSelectedStores] = useState<string[]>([])

  useEffect(() => {
    Promise.all([
      listAgents().catch(() => [] as AgentResponse[]),
      listProjects().catch(() => [] as Project[]),
      listMemoryStores().catch(() => [] as MemoryStore[]),
    ]).then(([a, p, m]) => {
      setAgents(a)
      setProjects(p)
      setMemoryStores(m)
      const def = a.find((x) => x.is_default)
      if (def) setAgentId(def.id)
      setLoading(false)
    })
  }, [])

  // keep agentId in sync with defaultAgent once agents load
  useEffect(() => {
    if (defaultAgent && agentId === '') setAgentId(defaultAgent.id)
  }, [defaultAgent])

  function toggleStore(id: string) {
    setSelectedStores((prev) => prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id])
  }

  function handleSubmit() {
    onCreate(
      projectId || undefined,
      selectedStores.length > 0 ? selectedStores : undefined,
      agentId || undefined,
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center pointer-events-none">
      {/* Dialog */}
      <div className="bg-background border border-border rounded-xl shadow-2xl w-full max-w-md mx-4 flex flex-col max-h-[85vh] pointer-events-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border flex-shrink-0">
          <h2 className="text-base font-semibold text-foreground">New Session</h2>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground transition-colors w-7 h-7 flex items-center justify-center rounded-md hover:bg-accent"
          >
            <svg viewBox="0 0 16 16" fill="none" className="w-4 h-4">
              <path d="M3 3l10 10M13 3L3 13" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
          {loading ? (
            <div className="text-sm text-muted-foreground py-6 text-center">Loading…</div>
          ) : (
            <>
              {/* Agent */}
              <div className="space-y-2">
                <label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Agent</label>
                <select
                  className="w-full h-8 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                  value={agentId}
                  onChange={(e) => setAgentId(e.target.value)}
                >
                  <option value="">— Use tenant default —</option>
                  {agents.length === 0 && (
                    <option disabled>No agents configured</option>
                  )}
                  {agents.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}{a.is_default ? ' (default)' : ''}
                    </option>
                  ))}
                </select>
              </div>

              {/* Project */}
              <div className="space-y-2">
                <label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Project</label>
                <select
                  className="w-full h-8 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                  value={projectId}
                  onChange={(e) => setProjectId(e.target.value)}
                >
                  <option value="">— No project —</option>
                  {projects.length === 0 && (
                    <option disabled>No projects yet</option>
                  )}
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              </div>

              {/* Memory Stores */}
              <div className="space-y-2">
                <label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Memory Stores</label>
                {memoryStores.length === 0 ? (
                  <p className="text-xs text-muted-foreground/60 py-1">No memory stores configured.</p>
                ) : (
                  <div className="rounded-md border border-border divide-y divide-border max-h-36 overflow-y-auto">
                    {memoryStores.map((s) => (
                      <label key={s.id} className="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-accent/40 transition-colors">
                        <input
                          type="checkbox"
                          className="w-3.5 h-3.5 accent-primary"
                          checked={selectedStores.includes(s.id)}
                          onChange={() => toggleStore(s.id)}
                        />
                        <span className="text-sm text-foreground/90 truncate flex-1">{s.name}</span>
                        {s.visibility === 'shared_tenant' && (
                          <span className="text-[10px] text-muted-foreground shrink-0 bg-muted px-1.5 py-0.5 rounded">shared</span>
                        )}
                      </label>
                    ))}
                  </div>
                )}
              </div>

            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 px-5 py-4 border-t border-border flex-shrink-0">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={creating}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleSubmit} disabled={creating || loading}>
            {creating ? 'Creating…' : 'Create Session'}
          </Button>
        </div>
      </div>
    </div>
  )
}

/**
 * Reconstruct a chat message list from raw harness history events.
 * Events are grouped into turns: each user.message starts a new turn;
 * agent activity (agent.message, agent.tool_use, agent.thinking) is
 * accumulated as segments within the current assistant message.
 */
function buildMessagesFromHistory(events: HarnessEvent[]): Message[] {
  const msgs: Message[] = []
  let assistantSegments: Message['segments'] = undefined

  function flushAssistant() {
    if (assistantSegments !== undefined) {
      msgs.push({
        id: `asst-${msgs.length}`,
        role: 'assistant',
        content: assistantSegments.filter((s) => s.type === 'token').map((s) => s.content).join(''),
        segments: assistantSegments,
      })
      assistantSegments = undefined
    }
  }

  for (const ev of events) {
    if (ev.type === 'user.message') {
      flushAssistant()
      msgs.push({ id: `user-${msgs.length}`, role: 'user', content: ev.content ?? '' })
    } else if (ev.type === 'agent.message') {
      if (!assistantSegments) assistantSegments = []
      const last = assistantSegments[assistantSegments.length - 1]
      if (last?.type === 'token') {
        assistantSegments[assistantSegments.length - 1] = { type: 'token', content: last.content + (ev.content ?? '') }
      } else {
        assistantSegments.push({ type: 'token', content: ev.content ?? '' })
      }
    } else if (ev.type === 'agent.tool_use') {
      if (!assistantSegments) assistantSegments = []
      assistantSegments.push({ type: 'tool_use', content: ev.description ?? ev.tool ?? '', tool: ev.tool, description: ev.description })
    } else if (ev.type === 'agent.thinking') {
      // skip — transient indicator, always appears before user.message in seq order
    }
  }
  flushAssistant()
  return msgs
}

function appendSegment(msg: Message, ev: SSEEvent): Message {
  const segments = [...(msg.segments ?? [])]
  const last = segments.length > 0 ? segments[segments.length - 1] : null

  if (ev.type === 'token') {
    if (last?.type === 'token') {
      segments[segments.length - 1] = { type: 'token', content: last.content + ev.content }
      return { ...msg, segments }
    }
  }

  if (ev.type === 'tool_use') {
    return { ...msg, segments: [...segments, { type: 'tool_use', content: ev.content, tool: ev.tool, description: ev.description }] }
  }

  return { ...msg, segments: [...segments, { type: ev.type as MessageSegment['type'], content: ev.content }] }
}


// ── resources + outputs panel ─────────────────────────────────────────────────

function ResourcesPanel({ sessionId }: { sessionId: string }) {
  const [resources, setResources] = useState<ResourceItem[]>([])
  const [outputs, setOutputs] = useState<OutputEntry[]>([])
  const [loadingRes, setLoadingRes] = useState(false)
  const [loadingOut, setLoadingOut] = useState(false)
  const [showAddGit, setShowAddGit] = useState(false)
  const [gitUrl, setGitUrl] = useState('')
  const [gitPath, setGitPath] = useState('')
  const [gitBranch, setGitBranch] = useState('')
  const [gitToken, setGitToken] = useState('')
  const [showAddUrl, setShowAddUrl] = useState(false)
  const [urlSrc, setUrlSrc] = useState('')
  const [urlPath, setUrlPath] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  function reloadResources() {
    setLoadingRes(true)
    listResources(sessionId).then(setResources).catch(() => {}).finally(() => setLoadingRes(false))
  }

  function reloadOutputs() {
    setLoadingOut(true)
    listOutputs(sessionId).then(setOutputs).catch(() => {}).finally(() => setLoadingOut(false))
  }

  useEffect(() => {
    reloadResources()
    reloadOutputs()
  }, [sessionId])

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const targetPath = file.name
    setAdding(true)
    setError('')
    try {
      await addFileResource(sessionId, file, targetPath)
      reloadResources()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setAdding(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  async function handleAddUrl(e: React.FormEvent) {
    e.preventDefault()
    if (!urlSrc || !urlPath) return
    setAdding(true)
    setError('')
    try {
      await addFileUrlResource(sessionId, urlSrc, urlPath)
      setShowAddUrl(false)
      setUrlSrc(''); setUrlPath('')
      reloadResources()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Add URL failed')
    } finally {
      setAdding(false)
    }
  }

  async function handleAddGit(e: React.FormEvent) {
    e.preventDefault()
    if (!gitUrl || !gitPath) return
    setAdding(true)
    setError('')
    try {
      await addGitResource(sessionId, gitUrl, gitPath, gitBranch || undefined, gitToken || undefined)
      setShowAddGit(false)
      setGitUrl(''); setGitPath(''); setGitBranch(''); setGitToken('')
      reloadResources()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Add git failed')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(resourceId: string) {
    try {
      await removeResource(sessionId, resourceId)
      setResources((prev) => prev.filter((r) => r.id !== resourceId))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Remove failed')
    }
  }

  async function handleDownload(path: string) {
    try {
      const blob = await downloadOutput(sessionId, path)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = path.split('/').pop() ?? path
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Download failed')
    }
  }

  return (
    <div className="w-72 flex-shrink-0 border-l border-border flex flex-col text-xs overflow-hidden">
      <ScrollArea className="flex-1">
        <div className="p-3 space-y-4">

          {error && (
            <div className="text-destructive bg-destructive/10 border border-destructive/20 rounded px-2 py-1.5">
              {error}
            </div>
          )}

          {/* Resources */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Resources</span>
              <div className="flex gap-1">
                <input ref={fileInputRef} type="file" className="hidden" onChange={handleFileUpload} />
                <Button variant="ghost" size="xs" className="h-5 px-1.5 text-[11px]"
                  onClick={() => { fileInputRef.current?.click() }} disabled={adding} title="Upload local file">
                  ↑ File
                </Button>
                <Button variant="ghost" size="xs" className="h-5 px-1.5 text-[11px]"
                  onClick={() => { setShowAddUrl((v) => !v); setShowAddGit(false) }} disabled={adding} title="Add file from URL">
                  ⊕ URL
                </Button>
                <Button variant="ghost" size="xs" className="h-5 px-1.5 text-[11px]"
                  onClick={() => { setShowAddGit((v) => !v); setShowAddUrl(false) }} disabled={adding} title="Clone git repository">
                  ⊕ Git
                </Button>
              </div>
            </div>

            {showAddUrl && (
              <form onSubmit={handleAddUrl} className="space-y-1.5 mb-2 p-2 bg-muted/40 rounded-md border border-border">
                <Input className="h-6 text-xs font-mono" placeholder="File URL *" value={urlSrc}
                  onChange={(e) => setUrlSrc(e.target.value)} required />
                <Input className="h-6 text-xs font-mono" placeholder="Target path *" value={urlPath}
                  onChange={(e) => setUrlPath(e.target.value)} required />
                <div className="flex gap-1">
                  <Button type="submit" size="xs" className="h-5 text-[11px]" disabled={adding}>
                    {adding ? '…' : 'Fetch'}
                  </Button>
                  <Button type="button" variant="ghost" size="xs" className="h-5 text-[11px]"
                    onClick={() => setShowAddUrl(false)}>Cancel</Button>
                </div>
              </form>
            )}

            {showAddGit && (
              <form onSubmit={handleAddGit} className="space-y-1.5 mb-2 p-2 bg-muted/40 rounded-md border border-border">
                <Input className="h-6 text-xs font-mono" placeholder="Repository URL *" value={gitUrl}
                  onChange={(e) => setGitUrl(e.target.value)} required />
                <Input className="h-6 text-xs font-mono" placeholder="Target path *" value={gitPath}
                  onChange={(e) => setGitPath(e.target.value)} required />
                <Input className="h-6 text-xs font-mono" placeholder="Branch (optional)" value={gitBranch}
                  onChange={(e) => setGitBranch(e.target.value)} />
                <Input className="h-6 text-xs font-mono" placeholder="Token (optional)" value={gitToken}
                  onChange={(e) => setGitToken(e.target.value)} type="password" />
                <div className="flex gap-1">
                  <Button type="submit" size="xs" className="h-5 text-[11px]" disabled={adding}>
                    {adding ? '…' : 'Clone'}
                  </Button>
                  <Button type="button" variant="ghost" size="xs" className="h-5 text-[11px]"
                    onClick={() => setShowAddGit(false)}>Cancel</Button>
                </div>
              </form>
            )}

            {loadingRes ? (
              <p className="text-muted-foreground text-[11px]">Loading…</p>
            ) : resources.length === 0 ? (
              <p className="text-muted-foreground text-[11px]">No resources</p>
            ) : (
              <div className="space-y-1">
                {resources.map((r) => (
                  <div key={r.id} className="flex items-start gap-1.5 p-1.5 rounded bg-muted/30 border border-border">
                    <div className="flex-1 min-w-0">
                      <p className="font-mono truncate text-foreground/80">{r.target_path}</p>
                      <p className="text-muted-foreground text-[10px]">
                        {r.type}{r.url ? ` · ${r.url}` : ''}{r.branch ? `@${r.branch}` : ''}
                      </p>
                    </div>
                    <button
                      onClick={() => handleRemove(r.id)}
                      className="text-muted-foreground hover:text-destructive flex-shrink-0 mt-0.5"
                    >×</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator />

          {/* Outputs */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Outputs</span>
              <Button variant="ghost" size="xs" className="h-5 px-1.5 text-[11px]"
                onClick={reloadOutputs} disabled={loadingOut}>
                ↺
              </Button>
            </div>
            {loadingOut ? (
              <p className="text-muted-foreground text-[11px]">Loading…</p>
            ) : outputs.length === 0 ? (
              <p className="text-muted-foreground text-[11px]">No outputs</p>
            ) : (
              <div className="space-y-1">
                {outputs.map((o) => (
                  <button
                    key={o.path}
                    onClick={() => handleDownload(o.path)}
                    className="w-full text-left flex items-center gap-1.5 p-1.5 rounded bg-muted/30 border border-border hover:bg-accent transition-colors"
                  >
                    <div className="flex-1 min-w-0">
                      <p className="font-mono truncate text-foreground/80">{o.path}</p>
                      <p className="text-muted-foreground text-[10px]">{(o.size / 1024).toFixed(1)} KB</p>
                    </div>
                    <span className="text-muted-foreground text-[10px] flex-shrink-0">↓</span>
                  </button>
                ))}
              </div>
            )}
          </div>

        </div>
      </ScrollArea>
    </div>
  )
}

// ── page ──────────────────────────────────────────────────────────────────────

export default function SessionsPage() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const navigate = useNavigate()
  const { role } = useAuth()

  const [sessions, setSessions] = useState<SessionListItem[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [creating, setCreating] = useState(false)
  const [showNewForm, setShowNewForm] = useState(false)
  const [messages, setMessages] = useState<Message[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const [showResourcePanel, setShowResourcePanel] = useState(false)
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null)
  const [editingTitle, setEditingTitle] = useState('')
  const [sessionStatus, setSessionStatus] = useState<string>('idle')
  const [sessionInitError, setSessionInitError] = useState<string>('')

  useEffect(() => {
    listSessions().then(setSessions).catch(() => {})
    listProjects().then(setProjects).catch(() => {})
  }, [])

  // Load history and poll status when session changes
  useEffect(() => {
    setMessages([])
    setIsStreaming(false)
    setSessionStatus('idle')
    setSessionInitError('')
    if (!sessionId) return

    const ac = new AbortController()

    // Fetch session metadata to get current status
    getSession(sessionId).then((meta) => {
      if (ac.signal.aborted) return
      setSessionStatus(meta.status)
      setSessionInitError(meta.init_error ?? '')
    }).catch(() => {})

    // Fetch full event history and reconstruct messages
    getSessionHistory(sessionId, ac.signal).then((events) => {
      if (ac.signal.aborted) return
      setMessages(buildMessagesFromHistory(events))
    }).catch(() => {})

    return () => ac.abort()
  }, [sessionId])

  // Poll session status while initializing
  useEffect(() => {
    if (sessionStatus !== 'initializing' || !sessionId) return
    const timer = setInterval(() => {
      getSession(sessionId).then((meta) => {
        setSessionStatus(meta.status)
        setSessionInitError(meta.init_error ?? '')
        // Refresh session list item status too
        setSessions((prev) =>
          prev.map((s) => s.session_id === sessionId
            ? { ...s, status: meta.status, init_error: meta.init_error }
            : s,
          ),
        )
      }).catch(() => {})
    }, 2000)
    return () => clearInterval(timer)
  }, [sessionStatus, sessionId])

  async function handleCreate(projectId?: string, stores?: string[], agentId?: string) {
    if (creating) return
    setCreating(true)
    try {
      const { session_id } = await createSession({ project_id: projectId, memory_stores: stores, agent_id: agentId })
      setSessions((prev) => [
        { session_id, project_id: projectId, status: 'initializing', created_at: new Date().toISOString() },
        ...prev,
      ])
      setShowNewForm(false)
      navigate(`/chat/${session_id}`)
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    try { await deleteSession(id) } catch { /* best effort */ }
    setSessions((prev) => prev.filter((s) => s.session_id !== id))
    if (id === sessionId) navigate('/')
  }

  async function handleSend(text: string, mode: 'default' | 'plan') {
    if (!sessionId || isStreaming) return

    const userMsg: Message = { id: crypto.randomUUID(), role: 'user', content: text }
    const assistantMsg: Message = { id: crypto.randomUUID(), role: 'assistant', content: '', segments: [], streaming: true }

    setMessages((prev) => [...prev, userMsg, assistantMsg])
    setIsStreaming(true)

    try {
      for await (const ev of streamRun(sessionId, text, mode)) {
        if (ev.type === 'title') {
          setSessions((prev) =>
            prev.map((s) => s.session_id === sessionId ? { ...s, title: ev.content } : s),
          )
          continue
        }
        setMessages((prev) =>
          prev.map((m) => m.id === assistantMsg.id ? appendSegment(m, ev) : m),
        )
      }
      setMessages((prev) =>
        prev.map((m) => m.id === assistantMsg.id ? { ...m, streaming: false } : m),
      )
    } catch (err) {
      const errText = err instanceof Error ? err.message : 'An error occurred'
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMsg.id ? { ...m, segments: undefined, content: errText, streaming: false, error: true } : m,
        ),
      )
    } finally {
      setIsStreaming(false)
    }
  }

  async function handleCommand(command: string) {
    if (!sessionId) return
    if (command === '/clear') {
      try {
        await clearHistory(sessionId)
      } catch { /* best-effort */ }
      setMessages([])
    }
  }

  async function handleSaveTitle(sid: string, title: string) {
    const trimmed = title.trim()
    setEditingSessionId(null)
    if (!trimmed) return
    try {
      await updateSessionTitle(sid, trimmed)
      setSessions((prev) => prev.map((s) => s.session_id === sid ? { ...s, title: trimmed } : s))
    } catch { /* best effort */ }
  }

  function projectName(projectId?: string) {
    if (!projectId) return null
    return projects.find((p) => p.id === projectId)?.name ?? null
  }

  return (
    <AppLayout>
      <div className="flex h-full overflow-hidden">

        {/* ── Session list panel ───────────────────────────────────────── */}
        <div className="w-64 flex-shrink-0 flex flex-col border-r border-border">
          <div className="px-4 py-3 flex items-center justify-between flex-shrink-0">
            <span className="text-sm font-semibold text-foreground">Sessions</span>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setShowNewForm(true)}
              disabled={creating}
              title="New session"
              className="w-7 h-7 text-muted-foreground hover:text-foreground"
            >
              <svg viewBox="0 0 16 16" fill="none" className="w-4 h-4">
                <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </Button>
          </div>

          <Separator />

          <ScrollArea className="flex-1">
            {sessions.length === 0 ? (
              <div className="px-4 py-10 text-center">
                <p className="text-muted-foreground text-xs mb-3">No sessions yet</p>
                <Button variant="outline" size="sm" onClick={() => setShowNewForm(true)} disabled={creating}>
                  + New Session
                </Button>
              </div>
            ) : (
              <div className="p-2 space-y-3 w-full">
                {(() => {
                  // Group by project, ungrouped last
                  const grouped = new Map<string, SessionListItem[]>()
                  const ungrouped: SessionListItem[] = []
                  for (const s of sessions) {
                    if (s.project_id) {
                      const g = grouped.get(s.project_id) ?? []
                      g.push(s)
                      grouped.set(s.project_id, g)
                    } else {
                      ungrouped.push(s)
                    }
                  }

                  function renderSession(s: SessionListItem) {
                    const isEditing = editingSessionId === s.session_id
                    const displayTitle = s.title || new Date(s.created_at).toLocaleString()
                    const hasError = s.status === 'error' || !!s.init_error
                    return (
                      <div
                        key={s.session_id}
                        onClick={() => !isEditing && navigate(`/chat/${s.session_id}`)}
                        className={cn(
                          'group w-full flex items-center gap-1 px-3 py-2 rounded-md cursor-pointer transition-colors',
                          s.session_id === sessionId
                            ? 'bg-accent text-accent-foreground'
                            : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground',
                        )}
                        title={s.init_error ?? undefined}
                      >
                        {isEditing ? (
                          <input
                            autoFocus
                            className="flex-1 min-w-0 w-0 text-xs font-medium bg-transparent border-b border-primary outline-none text-foreground"
                            value={editingTitle}
                            onChange={(e) => setEditingTitle(e.target.value)}
                            onBlur={() => handleSaveTitle(s.session_id, editingTitle)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') handleSaveTitle(s.session_id, editingTitle)
                              if (e.key === 'Escape') setEditingSessionId(null)
                              e.stopPropagation()
                            }}
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : (
                          <div className="flex-1 min-w-0 w-0">
                            <p className={cn('text-xs font-medium truncate', hasError ? 'text-destructive' : 'text-foreground')}>
                              {displayTitle}
                            </p>
                            {hasError && (
                              <p className="text-[10px] text-destructive/70 truncate">
                                {s.init_error ?? 'Init error'}
                              </p>
                            )}
                          </div>
                        )}
                        <div className="flex-shrink-0 flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                          <button
                            title="重命名"
                            onClick={(e) => {
                              e.stopPropagation()
                              setEditingSessionId(s.session_id)
                              setEditingTitle(s.title ?? '')
                            }}
                            className="w-5 h-5 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors"
                          >
                            <svg viewBox="0 0 16 16" fill="none" className="w-3 h-3">
                              <path d="M11.5 2.5a1.5 1.5 0 0 1 2.12 2.12L5 13.25 2 14l.75-3L11.5 2.5z" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round"/>
                            </svg>
                          </button>
                          <button
                            onClick={(e) => handleDelete(s.session_id, e)}
                            className="w-5 h-5 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors"
                          >
                            ×
                          </button>
                        </div>
                      </div>
                    )
                  }

                  const sections: ReactNode[] = []

                  for (const [pid, group] of grouped) {
                    const name = projects.find((p) => p.id === pid)?.name ?? pid
                    sections.push(
                      <div key={pid}>
                        <p className="px-3 mb-0.5 text-[10px] font-semibold uppercase tracking-wide text-primary/70 truncate">
                          {name}
                        </p>
                        <div className="space-y-0.5">{group.map(renderSession)}</div>
                      </div>
                    )
                  }

                  if (ungrouped.length > 0) {
                    sections.push(
                      <div key="__ungrouped">
                        {grouped.size > 0 && (
                          <p className="px-3 mb-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground/60 truncate">
                            No project
                          </p>
                        )}
                        <div className="space-y-0.5">{ungrouped.map(renderSession)}</div>
                      </div>
                    )
                  }

                  return sections
                })()}
              </div>
            )}
          </ScrollArea>
        </div>

        {/* ── Chat window panel ────────────────────────────────────────── */}
        <div className="flex-1 flex flex-col min-w-0">
          {sessionId ? (
            <>
              <div className="border-b border-border px-6 py-3 flex items-center gap-3 flex-shrink-0">
                <div className="w-2 h-2 rounded-full bg-primary flex-shrink-0" />
                <span className="text-muted-foreground text-xs font-mono truncate">{sessionId}</span>
                {projectName(sessions.find((s) => s.session_id === sessionId)?.project_id) && (
                  <span className="text-xs text-primary/70 font-medium ml-2 shrink-0">
                    {projectName(sessions.find((s) => s.session_id === sessionId)?.project_id)}
                  </span>
                )}
                <Button
                  variant={showResourcePanel ? 'secondary' : 'ghost'}
                  size="sm"
                  className="ml-auto h-7 px-2.5 text-xs gap-1.5 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowResourcePanel((v) => !v)}
                >
                  <svg viewBox="0 0 16 16" fill="none" className="w-3.5 h-3.5 flex-shrink-0" stroke="currentColor" strokeWidth="1.5">
                    <path d="M2 4h12M2 8h8M2 12h10" strokeLinecap="round" />
                  </svg>
                  Resources
                </Button>
              </div>

              {role === 'viewer' && (
                <div className="px-4 py-2 bg-muted border-b text-sm text-muted-foreground text-center flex-shrink-0">
                  只读模式 — 您的角色无法发送消息
                </div>
              )}

              {sessionStatus === 'initializing' && (
                <div className="px-4 py-2 bg-muted/60 border-b text-xs text-muted-foreground text-center flex-shrink-0 flex items-center justify-center gap-2">
                  <span className="inline-block w-2 h-2 rounded-full bg-yellow-500 animate-pulse" />
                  Initializing session environment… please wait
                </div>
              )}

              {sessionStatus === 'init_failed' && (
                <div className="px-4 py-2 bg-destructive/10 border-b border-destructive/30 text-xs text-destructive text-center flex-shrink-0">
                  Session initialization failed{sessionInitError ? `: ${sessionInitError}` : ''}. You may still send messages.
                </div>
              )}

              <div className="flex flex-1 min-h-0">
                <div className="flex flex-col flex-1 min-w-0">
                  <MessageList messages={messages} />
                  <MessageInput
                    onSend={handleSend}
                    onCommand={handleCommand}
                    disabled={isStreaming || role === 'viewer' || sessionStatus === 'initializing'}
                    showModeToggle={role !== 'viewer'}
                  />
                </div>
                {showResourcePanel && <ResourcesPanel sessionId={sessionId} />}
              </div>
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center space-y-4">
                <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mx-auto">
                  <svg viewBox="0 0 24 24" fill="none" className="w-6 h-6 text-muted-foreground" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H8.25m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H12m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 01-2.555-.337A5.972 5.972 0 015.41 20.97a5.969 5.969 0 01-.474-.065 4.48 4.48 0 00.978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25z" />
                  </svg>
                </div>
                <p className="text-muted-foreground text-sm">Select a session or create a new one</p>
                <Button onClick={() => setShowNewForm(true)} disabled={creating} variant="outline" size="sm">
                  + New Session
                </Button>
              </div>
            </div>
          )}
        </div>

      </div>

      {showNewForm && (
        <NewSessionModal
          creating={creating}
          onCreate={handleCreate}
          onClose={() => setShowNewForm(false)}
        />
      )}
    </AppLayout>
  )
}
