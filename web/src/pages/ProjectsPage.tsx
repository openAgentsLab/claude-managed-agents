import { useState, useEffect, type FormEvent } from 'react'
import AppLayout from '@/components/AppLayout'
import {
  listProjects,
  createProject,
  updateProject,
  deleteProject,
  type Project,
  type CreateProjectRequest,
} from '@/api/projects'
import { listEnvironments, type Environment } from '@/api/environments'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

// ── env var editor ────────────────────────────────────────────────────────────

function EnvEditor({
  value,
  onChange,
}: {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
}) {
  const entries = Object.entries(value)
  return (
    <div className="space-y-1.5">
      {entries.map(([k, v], i) => (
        <div key={i} className="flex gap-1.5">
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={k}
            placeholder="KEY"
            onChange={(e) => {
              const next = { ...value }
              delete next[k]
              next[e.target.value] = v
              onChange(next)
            }}
          />
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={v}
            placeholder="value"
            onChange={(e) => onChange({ ...value, [k]: e.target.value })}
          />
          <Button
            type="button"
            variant="ghost"
            size="xs"
            className="text-muted-foreground hover:text-destructive px-1.5"
            onClick={() => {
              const next = { ...value }
              delete next[k]
              onChange(next)
            }}
          >
            ×
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="xs"
        onClick={() => onChange({ ...value, '': '' })}
        className="text-xs h-6"
      >
        + Add var
      </Button>
    </div>
  )
}

// ── project form ──────────────────────────────────────────────────────────────

interface ProjectFormProps {
  initial?: Project
  environments: Environment[]
  saving: boolean
  onSave: (req: CreateProjectRequest) => void
  onCancel: () => void
}

function ProjectForm({ initial, environments, saving, onSave, onCancel }: ProjectFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [gitURL, setGitURL] = useState(initial?.git?.url ?? '')
  const [gitBranch, setGitBranch] = useState(initial?.git?.branch ?? '')
  const [gitUsername, setGitUsername] = useState(initial?.git?.username ?? '')
  const [gitToken, setGitToken] = useState('')
  const [envId, setEnvId] = useState(initial?.environment_id ?? '')
  const [envVars, setEnvVars] = useState<Record<string, string>>(initial?.env ?? {})

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    const req: CreateProjectRequest = {
      name: name.trim(),
      description: description || undefined,
      environment_id: envId || undefined,
      env: Object.keys(envVars).length > 0 ? envVars : undefined,
    }
    if (gitURL) {
      req.git = {
        url: gitURL,
        branch: gitBranch || undefined,
        username: gitUsername || undefined,
        token: gitToken || undefined,
      }
    }
    onSave(req)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input className="h-7 text-sm" value={name} onChange={(e) => setName(e.target.value)} placeholder="my-project" required />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Description</Label>
          <Input className="h-7 text-sm" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional description" />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-foreground/80">Git Repository</Label>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">URL</Label>
            <Input className="h-7 text-xs font-mono" value={gitURL} onChange={(e) => setGitURL(e.target.value)} placeholder="https://github.com/org/repo.git" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Branch</Label>
            <Input className="h-7 text-xs font-mono" value={gitBranch} onChange={(e) => setGitBranch(e.target.value)} placeholder="main" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Username</Label>
            <Input className="h-7 text-xs" value={gitUsername} onChange={(e) => setGitUsername(e.target.value)} placeholder="git-user" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Token {initial?.git?.url && '(leave blank to keep)'}</Label>
            <Input className="h-7 text-xs font-mono" type="password" value={gitToken} onChange={(e) => setGitToken(e.target.value)} placeholder={initial?.git?.url ? '••••••••' : 'ghp_…'} />
          </div>
        </div>
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">Environment</Label>
        {environments.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            No environments yet — create one in{' '}
            <a href="/settings/environments" className="underline hover:text-foreground">Environments</a>.
          </p>
        ) : (
          <select
            className="h-7 w-full rounded-md border border-input bg-background px-3 text-sm"
            value={envId}
            onChange={(e) => setEnvId(e.target.value)}
          >
            <option value="">None</option>
            {environments.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}{env.scope === 'tenant' ? ' (tenant)' : ''}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">Environment Variables</Label>
        <EnvEditor value={envVars} onChange={setEnvVars} />
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim()}>
          {saving ? 'Saving…' : initial ? 'Update' : 'Create Project'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

// ── page ──────────────────────────────────────────────────────────────────────

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [environments, setEnvironments] = useState<Environment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([listProjects(), listEnvironments()])
      .then(([projs, envs]) => {
        setProjects(projs)
        setEnvironments(envs)
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleCreate(req: CreateProjectRequest) {
    setSaving(true)
    setError('')
    try {
      const p = await createProject(req)
      setProjects((prev) => [p, ...prev])
      setShowCreate(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(id: string, req: CreateProjectRequest) {
    setSaving(true)
    setError('')
    try {
      const p = await updateProject(id, req)
      setProjects((prev) => prev.map((x) => (x.id === id ? p : x)))
      setEditId(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete project "${name}"?`)) return
    setError('')
    try {
      await deleteProject(id)
      setProjects((prev) => prev.filter((x) => x.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-3xl mx-auto space-y-5">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Projects</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Organize sessions with git repos, environments, and preset variables.
            </p>
          </div>
          {!showCreate && (
            <Button size="sm" onClick={() => { setShowCreate(true); setEditId(null) }}>
              + New Project
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showCreate && (
          <ProjectForm
            environments={environments}
            saving={saving}
            onSave={handleCreate}
            onCancel={() => setShowCreate(false)}
          />
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : projects.length === 0 && !showCreate ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No projects yet</div>
        ) : (
          <div className="space-y-2">
            {projects.map((p) => (
              <Card key={p.id}>
                <CardContent className="px-4 py-3">
                  {editId === p.id ? (
                    <ProjectForm
                      initial={p}
                      environments={environments}
                      saving={saving}
                      onSave={(req) => handleUpdate(p.id, req)}
                      onCancel={() => setEditId(null)}
                    />
                  ) : (
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-foreground">{p.name}</p>
                        {p.description && (
                          <p className="text-xs text-muted-foreground mt-0.5">{p.description}</p>
                        )}
                        <div className="flex flex-wrap gap-3 mt-1.5">
                          {p.git?.url && (
                            <span className="text-[11px] text-muted-foreground font-mono truncate max-w-xs">
                              git: {p.git.url}
                            </span>
                          )}
                          {p.environment_id && (
                            <span className="text-[11px] text-muted-foreground">
                              env: {environments.find((e) => e.id === p.environment_id)?.name ?? p.environment_id}
                            </span>
                          )}
                        </div>
                        <p className="text-[11px] text-muted-foreground/60 mt-1">
                          Updated {new Date(p.updated_at).toLocaleString()}
                        </p>
                      </div>
                      <div className="flex gap-1 shrink-0">
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => { setEditId(p.id); setShowCreate(false) }}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => handleDelete(p.id, p.name)}
                          className="text-muted-foreground hover:text-destructive"
                        >
                          Delete
                        </Button>
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
