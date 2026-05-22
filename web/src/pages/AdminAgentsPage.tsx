import { useState, useEffect, type FormEvent } from 'react'
import AppLayout from '@/components/AppLayout'
import {
  listAgents, createAgent, updateAgent, archiveAgent,
  type AgentResponse, type CreateAgentRequest, type UpdateAgentRequest,
} from '@/api/agents'
import { listMCPServers } from '@/api/mcp'
import { listSkills } from '@/api/skills'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

// ── Multi-select chips ────────────────────────────────────────────────────────

function MultiSelect({
  label,
  options,
  selected,
  onChange,
  variant = 'chips',
}: {
  label: string
  options: string[]
  selected: string[]
  onChange: (v: string[]) => void
  variant?: 'chips' | 'list'
}) {
  function toggle(opt: string) {
    onChange(selected.includes(opt) ? selected.filter((s) => s !== opt) : [...selected, opt])
  }

  return (
    <div className="space-y-1.5">
      <Label className="text-xs">{label}</Label>
      {options.length === 0 ? (
        <p className="text-xs text-muted-foreground">None available</p>
      ) : variant === 'list' ? (
        <div className="rounded-md border border-border divide-y divide-border max-h-40 overflow-y-auto">
          {options.map((opt) => (
            <label key={opt} className="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-accent/40 transition-colors">
              <input
                type="checkbox"
                checked={selected.includes(opt)}
                onChange={() => toggle(opt)}
                className="w-3.5 h-3.5 accent-primary rounded"
              />
              <span className="text-sm text-foreground/90 truncate">{opt}</span>
            </label>
          ))}
        </div>
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {options.map((opt) => (
            <button
              key={opt}
              type="button"
              onClick={() => toggle(opt)}
              className={cn(
                'text-xs px-2 py-0.5 rounded-full border transition-colors',
                selected.includes(opt)
                  ? 'bg-primary text-primary-foreground border-primary'
                  : 'bg-transparent text-muted-foreground border-border hover:border-foreground/40',
              )}
            >
              {opt}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Agent form ────────────────────────────────────────────────────────────────

interface AgentFormProps {
  initial?: AgentResponse
  mcpOptions: string[]
  skillOptions: string[]
  agentOptions: string[]
  saving: boolean
  onSave: (req: CreateAgentRequest | UpdateAgentRequest) => void
  onCancel: () => void
}

function AgentForm({ initial, mcpOptions, skillOptions, agentOptions, saving, onSave, onCancel }: AgentFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [model, setModel] = useState(initial?.model ?? '')
  const [systemPrompt, setSystemPrompt] = useState(initial?.system_prompt ?? '')
  const [isDefault, setIsDefault] = useState(initial?.is_default ?? false)
  const [selectedMCPs, setSelectedMCPs] = useState<string[]>(initial?.mcp_server_names ?? [])
  const [selectedSkills, setSelectedSkills] = useState<string[]>(initial?.skill_names ?? [])
  const [selectedCallable, setSelectedCallable] = useState<string[]>(initial?.callable_agents ?? [])

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    onSave({
      name: name.trim(),
      description: description || undefined,
      model: model || undefined,
      system_prompt: systemPrompt || undefined,
      is_default: isDefault,
      mcp_server_names: selectedMCPs.length > 0 ? selectedMCPs : [],
      skill_names: selectedSkills.length > 0 ? selectedSkills : [],
      callable_agents: selectedCallable.length > 0 ? selectedCallable : [],
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input
            className="h-7 text-sm"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="my-agent"
            disabled={!!initial}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Model</Label>
          <Input
            className="h-7 text-sm"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="(tenant default)"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">Description</Label>
        <Input
          className="h-7 text-sm"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What this agent does"
        />
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">System Prompt</Label>
        <Textarea
          className="font-mono text-xs min-h-[120px] resize-y"
          value={systemPrompt}
          onChange={(e) => setSystemPrompt(e.target.value)}
          placeholder="You are a helpful assistant…"
        />
      </div>

      <MultiSelect
        label="MCP Servers"
        options={mcpOptions}
        selected={selectedMCPs}
        onChange={setSelectedMCPs}
      />

      <MultiSelect
        label="Skills"
        options={skillOptions}
        selected={selectedSkills}
        onChange={setSelectedSkills}
      />

      {agentOptions.length > 0 && (
        <MultiSelect
          label="Callable Agents"
          options={agentOptions}
          selected={selectedCallable}
          onChange={setSelectedCallable}
          variant="list"
        />
      )}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => setIsDefault(!isDefault)}
          className={cn(
            'relative inline-flex h-4 w-8 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            isDefault ? 'bg-primary' : 'bg-muted-foreground/30',
          )}
        >
          <span
            className={cn(
              'inline-block size-3 rounded-full bg-white shadow transition-transform',
              isDefault ? 'translate-x-4' : 'translate-x-0.5',
            )}
          />
        </button>
        <span className="text-xs text-muted-foreground">Default agent</span>
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim()}>
          {saving ? 'Saving…' : initial ? 'Update' : 'Create Agent'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function AdminAgentsPage() {
  const [agents, setAgents] = useState<AgentResponse[]>([])
  const [mcpOptions, setMcpOptions] = useState<string[]>([])
  const [skillOptions, setSkillOptions] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)

  async function reload() {
    const list = await listAgents()
    setAgents(list)
    return list
  }

  useEffect(() => {
    Promise.all([
      reload(),
      listMCPServers().then((s) => setMcpOptions(s.map((m) => m.name))),
      listSkills().then((s) => setSkillOptions(s.map((sk) => sk.name))),
    ])
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleCreate(req: CreateAgentRequest | UpdateAgentRequest) {
    setSaving(true)
    setError('')
    try {
      await createAgent(req as CreateAgentRequest)
      await reload()
      setShowCreate(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(id: string, req: CreateAgentRequest | UpdateAgentRequest) {
    setSaving(true)
    setError('')
    try {
      await updateAgent(id, req as UpdateAgentRequest)
      await reload()
      setEditingId(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  async function handleArchive(id: string, name: string) {
    if (!confirm(`Archive agent "${name}"? It will no longer appear in session creation.`)) return
    setError('')
    try {
      await archiveAgent(id)
      setAgents((prev) => prev.filter((a) => a.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to archive')
    }
  }

  const agentNames = agents.map((a) => a.name)

  return (
    <AppLayout>
      <div className="p-6 max-w-3xl mx-auto space-y-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Agents</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Configure agents available for sessions. The default agent is used when none is specified.
            </p>
          </div>
          {!showCreate && (
            <Button size="sm" onClick={() => { setShowCreate(true); setEditingId(null) }}>
              + New Agent
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showCreate && (
          <AgentForm
            mcpOptions={mcpOptions}
            skillOptions={skillOptions}
            agentOptions={agentNames}
            saving={saving}
            onSave={handleCreate}
            onCancel={() => setShowCreate(false)}
          />
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : agents.length === 0 ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No agents configured</div>
        ) : (
          <div className="space-y-2">
            {agents.map((agent) => (
              <Card key={agent.id} className="overflow-hidden">
                <CardContent className="p-0">
                  {editingId === agent.id ? (
                    <div className="p-4">
                      <AgentForm
                        initial={agent}
                        mcpOptions={mcpOptions}
                        skillOptions={skillOptions}
                        agentOptions={agentNames.filter((n) => n !== agent.name)}
                        saving={saving}
                        onSave={(req) => handleUpdate(agent.id, req)}
                        onCancel={() => setEditingId(null)}
                      />
                    </div>
                  ) : (
                    <div className="flex items-start gap-3 px-4 py-3">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-sm font-medium text-foreground">{agent.name}</span>
                          {agent.is_default && (
                            <Badge className="text-[10px]">default</Badge>
                          )}
                          {agent.model && (
                            <Badge variant="outline" className="text-[10px] font-mono">{agent.model}</Badge>
                          )}
                        </div>
                        {agent.description && (
                          <p className="text-xs text-muted-foreground mt-0.5">{agent.description}</p>
                        )}
                        <div className="flex flex-wrap gap-3 mt-1">
                          {(agent.mcp_server_names?.length ?? 0) > 0 && (
                            <span className="text-[11px] text-muted-foreground">
                              {agent.mcp_server_names!.length} MCP server{agent.mcp_server_names!.length !== 1 ? 's' : ''}
                            </span>
                          )}
                          {(agent.skill_names?.length ?? 0) > 0 && (
                            <span className="text-[11px] text-muted-foreground">
                              {agent.skill_names!.length} skill{agent.skill_names!.length !== 1 ? 's' : ''}
                            </span>
                          )}
                          {(agent.callable_agents?.length ?? 0) > 0 && (
                            <span className="text-[11px] text-muted-foreground">
                              {agent.callable_agents!.length} callable agent{agent.callable_agents!.length !== 1 ? 's' : ''}
                            </span>
                          )}
                        </div>
                        <p className="text-[11px] text-muted-foreground/50 mt-1">v{agent.version}</p>
                      </div>
                      <div className="flex gap-1 shrink-0">
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => { setEditingId(agent.id); setShowCreate(false) }}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => handleArchive(agent.id, agent.name)}
                          className="text-muted-foreground hover:text-destructive"
                        >
                          Archive
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
