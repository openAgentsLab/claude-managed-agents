import { useState, useEffect, type FormEvent } from 'react'
import AppLayout from '@/components/AppLayout'
import {
  listTenantEnvironments,
  createTenantEnvironment,
  updateTenantEnvironment,
  deleteTenantEnvironment,
  type Environment,
  type EnvironmentRequest,
} from '@/api/environments'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

function EnvVarEditor({
  value,
  onChange,
}: {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
}) {
  return (
    <div className="space-y-1.5">
      {Object.entries(value).map(([k, v], i) => (
        <div key={i} className="flex gap-1.5">
          <Input className="h-7 text-xs font-mono flex-1" value={k} placeholder="KEY"
            onChange={(e) => { const n = { ...value }; delete n[k]; onChange({ ...n, [e.target.value]: v }) }} />
          <Input className="h-7 text-xs font-mono flex-1" value={v} placeholder="value"
            onChange={(e) => onChange({ ...value, [k]: e.target.value })} />
          <Button type="button" variant="ghost" size="xs" className="text-muted-foreground hover:text-destructive px-1.5"
            onClick={() => { const n = { ...value }; delete n[k]; onChange(n) }}>×</Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="xs" onClick={() => onChange({ ...value, '': '' })} className="text-xs h-6">
        + Add var
      </Button>
    </div>
  )
}

function EnvironmentForm({
  initial,
  saving,
  onSave,
  onCancel,
}: {
  initial?: Environment
  saving: boolean
  onSave: (req: EnvironmentRequest) => void
  onCancel: () => void
}) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [pip, setPip] = useState((initial?.packages?.pip ?? []).join(', '))
  const [npm, setNpm] = useState((initial?.packages?.npm ?? []).join(', '))
  const [apt, setApt] = useState((initial?.packages?.apt ?? []).join(', '))
  const [cargo, setCargo] = useState((initial?.packages?.cargo ?? []).join(', '))
  const [netMode, setNetMode] = useState(initial?.networking?.mode ?? 'unrestricted')
  const [allowedHosts, setAllowedHosts] = useState((initial?.networking?.allowed_hosts ?? []).join(', '))
  const [envVars, setEnvVars] = useState<Record<string, string>>(initial?.env ?? {})

  function splitList(s: string) {
    return s.split(',').map((x) => x.trim()).filter(Boolean)
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    onSave({
      name: name.trim(),
      description: description || undefined,
      packages: { pip: splitList(pip), npm: splitList(npm), apt: splitList(apt), cargo: splitList(cargo) },
      networking: { mode: netMode, allowed_hosts: netMode === 'limited' ? splitList(allowedHosts) : undefined },
      env: Object.keys(envVars).length > 0 ? envVars : undefined,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input className="h-7 text-sm" value={name} onChange={(e) => setName(e.target.value)} placeholder="base-python" required />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Description</Label>
          <Input className="h-7 text-sm" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-foreground/80">Packages</Label>
        <div className="grid grid-cols-2 gap-3">
          {[['Pip', pip, setPip], ['Npm', npm, setNpm], ['Apt', apt, setApt], ['Cargo', cargo, setCargo]].map(([label, val, setter]) => (
            <div key={label as string} className="space-y-1">
              <Label className="text-[11px] text-muted-foreground">{label as string}</Label>
              <Input className="h-7 text-xs font-mono" value={val as string}
                onChange={(e) => (setter as (v: string) => void)(e.target.value)} placeholder="pkg1, pkg2, …" />
            </div>
          ))}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-foreground/80">Networking</Label>
        <div className="flex items-center gap-3">
          <select className="h-7 rounded-md border border-input bg-background px-3 text-sm"
            value={netMode} onChange={(e) => setNetMode(e.target.value)}>
            <option value="unrestricted">Unrestricted</option>
            <option value="limited">Limited</option>
          </select>
          {netMode === 'limited' && (
            <Input className="h-7 text-xs font-mono flex-1" value={allowedHosts}
              onChange={(e) => setAllowedHosts(e.target.value)} placeholder="github.com, pypi.org, …" />
          )}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-foreground/80">Environment Variables</Label>
        <EnvVarEditor value={envVars} onChange={setEnvVars} />
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim()}>
          {saving ? 'Saving…' : initial ? 'Update' : 'Create Environment'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>Cancel</Button>
      </div>
    </form>
  )
}

export default function AdminEnvironmentsPage() {
  const [envs, setEnvs] = useState<Environment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)

  function reload() {
    return listTenantEnvironments().then(setEnvs)
  }

  useEffect(() => {
    reload().catch((e) => setError(e.message)).finally(() => setLoading(false))
  }, [])

  async function handleCreate(req: EnvironmentRequest) {
    setSaving(true)
    setError('')
    try {
      await createTenantEnvironment(req)
      await reload()
      setShowCreate(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(id: string, req: EnvironmentRequest) {
    setSaving(true)
    setError('')
    try {
      await updateTenantEnvironment(id, req)
      await reload()
      setEditId(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete tenant environment "${name}"?`)) return
    setError('')
    try {
      await deleteTenantEnvironment(id)
      setEnvs((prev) => prev.filter((e) => e.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-5">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Tenant Environments</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Shared environments available to all tenant members.
            </p>
          </div>
          {!showCreate && (
            <Button size="sm" onClick={() => { setShowCreate(true); setEditId(null) }}>
              + New Environment
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showCreate && (
          <EnvironmentForm saving={saving} onSave={handleCreate} onCancel={() => setShowCreate(false)} />
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : envs.length === 0 && !showCreate ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No tenant environments yet</div>
        ) : (
          <div className="space-y-2">
            {envs.map((env) => (
              <Card key={env.id}>
                <CardContent className="px-4 py-3">
                  {editId === env.id ? (
                    <EnvironmentForm
                      initial={env}
                      saving={saving}
                      onSave={(req) => handleUpdate(env.id, req)}
                      onCancel={() => setEditId(null)}
                    />
                  ) : (
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-foreground">{env.name}</p>
                        {env.description && (
                          <p className="text-xs text-muted-foreground mt-0.5">{env.description}</p>
                        )}
                        <div className="flex flex-wrap gap-3 mt-1">
                          {env.networking?.mode === 'limited' && (
                            <span className="text-[11px] text-muted-foreground">network: limited</span>
                          )}
                          {env.env && Object.keys(env.env).length > 0 && (
                            <span className="text-[11px] text-muted-foreground">
                              {Object.keys(env.env).length} env var{Object.keys(env.env).length !== 1 ? 's' : ''}
                            </span>
                          )}
                        </div>
                        <p className="text-[11px] text-muted-foreground/60 mt-1">
                          Updated {new Date(env.updated_at).toLocaleString()}
                        </p>
                      </div>
                      <div className="flex gap-1 shrink-0">
                        <Button variant="ghost" size="xs" onClick={() => { setEditId(env.id); setShowCreate(false) }}
                          className="text-muted-foreground hover:text-foreground">Edit</Button>
                        <Button variant="ghost" size="xs" onClick={() => handleDelete(env.id, env.name)}
                          className="text-muted-foreground hover:text-destructive">Delete</Button>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </AppLayout>
  )
}
