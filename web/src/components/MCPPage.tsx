import { useState, useEffect, type FormEvent } from 'react'
import { type MCPServer, type UpsertMCPServerRequest } from '@/api/mcp'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'

// ── Key-value editor ──────────────────────────────────────────────────────────

function KVEditor({
  value,
  onChange,
  placeholder,
}: {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
  placeholder?: string
}) {
  const pairs = Object.entries(value)

  function updateKey(i: number, newKey: string) {
    const next: Record<string, string> = {}
    pairs.forEach(([k, v], idx) => { next[idx === i ? newKey : k] = v })
    onChange(next)
  }

  function updateVal(i: number, newVal: string) {
    const next: Record<string, string> = {}
    pairs.forEach(([k, v], idx) => { next[k] = idx === i ? newVal : v })
    onChange(next)
  }

  function remove(i: number) {
    const next: Record<string, string> = {}
    pairs.forEach(([k, v], idx) => { if (idx !== i) next[k] = v })
    onChange(next)
  }

  function add() {
    onChange({ ...value, '': '' })
  }

  return (
    <div className="space-y-1.5">
      {pairs.map(([k, v], i) => (
        <div key={i} className="flex gap-1.5 items-center">
          <Input
            className="h-7 text-xs font-mono w-36 shrink-0"
            value={k}
            placeholder="key"
            onChange={(e) => updateKey(i, e.target.value)}
          />
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={v}
            placeholder={placeholder ?? 'value'}
            onChange={(e) => updateVal(i, e.target.value)}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={() => remove(i)}
            className="text-muted-foreground hover:text-destructive"
          >
            ×
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="xs" onClick={add}>
        + Add
      </Button>
    </div>
  )
}

// ── MCP server form ───────────────────────────────────────────────────────────

interface FormState {
  name: string
  type: string
  command: string
  args: string
  url: string
  env: Record<string, string>
  headers: Record<string, string>
  disabled: boolean
}

function emptyForm(): FormState {
  return { name: '', type: 'stdio', command: '', args: '', url: '', env: {}, headers: {}, disabled: false }
}

function serverToForm(s: MCPServer): FormState {
  return {
    name: s.name,
    type: s.type || 'stdio',
    command: s.command ?? '',
    args: (s.args ?? []).join(' '),
    url: s.url ?? '',
    env: s.env ?? {},
    headers: s.headers ?? {},
    disabled: s.disabled,
  }
}

function formToRequest(f: FormState): UpsertMCPServerRequest {
  const req: UpsertMCPServerRequest = { name: f.name, type: f.type, disabled: f.disabled }
  if (f.type === 'stdio') {
    if (f.command) req.command = f.command
    if (f.args.trim()) req.args = f.args.trim().split(/\s+/)
  } else {
    if (f.url) req.url = f.url
    if (Object.keys(f.headers).length) req.headers = f.headers
  }
  if (Object.keys(f.env).length) req.env = f.env
  return req
}

interface MCPServerFormProps {
  initial?: FormState
  isNew?: boolean
  saving: boolean
  onSave: (req: UpsertMCPServerRequest) => void
  onCancel: () => void
}

function MCPServerForm({ initial, isNew, saving, onSave, onCancel }: MCPServerFormProps) {
  const [form, setForm] = useState<FormState>(initial ?? emptyForm())

  function set<K extends keyof FormState>(k: K, v: FormState[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    onSave(formToRequest(form))
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input
            className="h-7 text-sm"
            value={form.name}
            onChange={(e) => set('name', e.target.value)}
            placeholder="my-server"
            disabled={!isNew}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Type</Label>
          <select
            className="h-7 w-full rounded-md border border-input bg-background px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            value={form.type}
            onChange={(e) => set('type', e.target.value)}
          >
            <option value="stdio">stdio</option>
            <option value="sse">sse</option>
            <option value="http">http</option>
          </select>
        </div>
      </div>

      {form.type === 'stdio' ? (
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">Command</Label>
            <Input
              className="h-7 text-sm font-mono"
              value={form.command}
              onChange={(e) => set('command', e.target.value)}
              placeholder="npx"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Args (space-separated)</Label>
            <Input
              className="h-7 text-sm font-mono"
              value={form.args}
              onChange={(e) => set('args', e.target.value)}
              placeholder="-y @modelcontextprotocol/server-github"
            />
          </div>
        </div>
      ) : (
        <div className="space-y-1.5">
          <Label className="text-xs">URL</Label>
          <Input
            className="h-7 text-sm font-mono"
            value={form.url}
            onChange={(e) => set('url', e.target.value)}
            placeholder="https://..."
          />
        </div>
      )}

      <div className="space-y-1.5">
        <Label className="text-xs">Environment Variables</Label>
        <KVEditor
          value={form.env}
          onChange={(v) => set('env', v)}
          placeholder="value or vault:MY_SECRET"
        />
      </div>

      {form.type !== 'stdio' && (
        <div className="space-y-1.5">
          <Label className="text-xs">Headers</Label>
          <KVEditor value={form.headers} onChange={(v) => set('headers', v)} />
        </div>
      )}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => set('disabled', !form.disabled)}
          className={cn(
            'relative inline-flex h-4 w-8 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            form.disabled ? 'bg-muted-foreground/30' : 'bg-primary',
          )}
        >
          <span
            className={cn(
              'inline-block size-3 rounded-full bg-white shadow transition-transform',
              form.disabled ? 'translate-x-0.5' : 'translate-x-4',
            )}
          />
        </button>
        <span className="text-xs text-muted-foreground">{form.disabled ? 'Disabled' : 'Enabled'}</span>
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !form.name.trim()}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

// ── Read-only detail view ─────────────────────────────────────────────────────

function MCPServerDetail({ server }: { server: MCPServer }) {
  const envEntries = Object.entries(server.env ?? {})
  const headerEntries = Object.entries(server.headers ?? {})
  const hasDetail = (server.type === 'stdio' ? server.command : server.url) || envEntries.length > 0 || headerEntries.length > 0

  return (
    <div className="border-t border-border p-4 space-y-3 bg-muted/20">
      {server.type === 'stdio' && server.command && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-0.5">Command</p>
          <p className="text-xs font-mono">{[server.command, ...(server.args ?? [])].join(' ')}</p>
        </div>
      )}
      {server.type !== 'stdio' && server.url && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-0.5">URL</p>
          <p className="text-xs font-mono">{server.url}</p>
        </div>
      )}
      {envEntries.length > 0 && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-0.5">Environment</p>
          <div className="space-y-0.5">
            {envEntries.map(([k, v]) => (
              <p key={k} className="text-xs font-mono">
                <span className="text-muted-foreground">{k}</span>=<span>{v}</span>
              </p>
            ))}
          </div>
        </div>
      )}
      {headerEntries.length > 0 && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-0.5">Headers</p>
          <div className="space-y-0.5">
            {headerEntries.map(([k, v]) => (
              <p key={k} className="text-xs font-mono">
                <span className="text-muted-foreground">{k}</span>: {v}
              </p>
            ))}
          </div>
        </div>
      )}
      {!hasDetail && (
        <p className="text-xs text-muted-foreground">No additional configuration</p>
      )}
    </div>
  )
}

// ── MCPPage ───────────────────────────────────────────────────────────────────

export interface MCPPageProps {
  title: string
  description?: string
  listFn: () => Promise<MCPServer[]>
  upsertFn?: (req: UpsertMCPServerRequest) => Promise<void>
  deleteFn?: (name: string) => Promise<void>
  readOnly?: boolean
}

export default function MCPPage({ title, description, listFn, upsertFn, deleteFn, readOnly }: MCPPageProps) {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)

  useEffect(() => {
    listFn()
      .then(setServers)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleSave(req: UpsertMCPServerRequest) {
    if (!upsertFn) return
    setSaving(true)
    setError('')
    try {
      await upsertFn(req)
      const list = await listFn()
      setServers(list)
      setShowForm(false)
      setEditing(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(name: string) {
    if (!deleteFn) return
    if (!confirm(`Delete "${name}"?`)) return
    setError('')
    try {
      await deleteFn(name)
      setServers((s) => s.filter((x) => x.name !== name))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-5">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-lg font-semibold text-foreground">{title}</h1>
          {description && <p className="text-sm text-muted-foreground mt-0.5">{description}</p>}
        </div>
        {!readOnly && !showForm && (
          <Button size="sm" onClick={() => setShowForm(true)}>
            + Add Server
          </Button>
        )}
      </div>

      {error && (
        <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
          {error}
        </div>
      )}

      {showForm && (
        <MCPServerForm
          isNew
          saving={saving}
          onSave={handleSave}
          onCancel={() => setShowForm(false)}
        />
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading…</div>
      ) : servers.length === 0 ? (
        <div className="text-center py-16 text-sm text-muted-foreground">No MCP servers configured</div>
      ) : (
        <div className="space-y-2">
          {servers.map((s) => (
            <Card key={s.name} className="overflow-hidden">
              <CardContent className="p-0">
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors"
                  onClick={() => setEditing(editing === s.name ? null : s.name)}
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-foreground">{s.name}</span>
                      <Badge variant="outline" className="text-[10px] font-mono">{s.type}</Badge>
                      {s.disabled && <Badge variant="secondary" className="text-[10px]">disabled</Badge>}
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5 truncate font-mono">
                      {s.command || s.url || '—'}
                    </p>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {!readOnly && (
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={(e) => { e.stopPropagation(); handleDelete(s.name) }}
                        className="text-muted-foreground hover:text-destructive"
                      >
                        Delete
                      </Button>
                    )}
                    <span className="text-muted-foreground/40 text-xs">{editing === s.name ? '▲' : '▼'}</span>
                  </div>
                </div>
                {editing === s.name && (
                  readOnly ? (
                    <MCPServerDetail server={s} />
                  ) : (
                    <div className="border-t border-border">
                      <MCPServerForm
                        initial={serverToForm(s)}
                        saving={saving}
                        onSave={handleSave}
                        onCancel={() => setEditing(null)}
                      />
                    </div>
                  )
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
