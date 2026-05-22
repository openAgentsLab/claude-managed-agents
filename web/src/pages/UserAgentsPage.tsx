import { useState, useEffect, type ReactNode } from 'react'
import AppLayout from '@/components/AppLayout'
import { listAgents, type AgentResponse } from '@/api/agents'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

function DetailSection({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground mb-1">{label}</p>
      {children}
    </div>
  )
}

function BadgeList({ items, variant = 'secondary' }: { items: string[]; variant?: 'secondary' | 'outline' }) {
  if (items.length === 0) return <p className="text-foreground/40 text-[11px]">—</p>
  return (
    <div className="flex flex-wrap gap-1">
      {items.map((n) => <Badge key={n} variant={variant} className="text-[10px]">{n}</Badge>)}
    </div>
  )
}

function AgentDetail({ agent }: { agent: AgentResponse }) {
  return (
    <div className="px-4 pb-4 pt-3 space-y-3 text-xs border-t border-border bg-muted/10">
      <div className="grid grid-cols-2 gap-3">
        <DetailSection label="Model">
          <p className="font-mono text-foreground/80">{agent.model || <span className="text-foreground/40">(tenant default)</span>}</p>
        </DetailSection>

        <div className="col-span-2">
          <DetailSection label="System Prompt">
            {agent.system_prompt
              ? <pre className="font-mono text-foreground/80 whitespace-pre-wrap bg-muted/40 rounded p-2 max-h-40 overflow-y-auto text-[11px]">{agent.system_prompt}</pre>
              : <p className="text-foreground/40 text-[11px]">—</p>}
          </DetailSection>
        </div>

        <DetailSection label="MCP Servers">
          <BadgeList items={agent.mcp_server_names ?? []} />
        </DetailSection>

        <DetailSection label="Skills">
          <BadgeList items={agent.skill_names ?? []} />
        </DetailSection>

        <div className="col-span-2">
          <DetailSection label="Callable Agents">
            <BadgeList items={agent.callable_agents ?? []} variant="outline" />
          </DetailSection>
        </div>
      </div>
    </div>
  )
}

export default function UserAgentsPage() {
  const [agents, setAgents] = useState<AgentResponse[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [expanded, setExpanded] = useState<string | null>(null)

  useEffect(() => {
    listAgents()
      .then(setAgents)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load agents'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <AppLayout>
      <div className="p-6 max-w-3xl mx-auto space-y-5">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Agents</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Agents available in your tenant. You can select an agent when creating a session.
          </p>
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : agents.length === 0 ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No agents configured</div>
        ) : (
          <div className="space-y-2">
            {agents.map((a) => (
              <Card key={a.id} className="overflow-hidden">
                <CardContent className="p-0">
                  <div
                    className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors"
                    onClick={() => setExpanded(expanded === a.id ? null : a.id)}
                  >
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-foreground">{a.name}</span>
                        {a.is_default && (
                          <Badge variant="default" className="text-[10px]">default</Badge>
                        )}
                      </div>
                      {a.description && (
                        <p className="text-xs text-muted-foreground truncate mt-0.5">{a.description}</p>
                      )}
                    </div>
                    <span className="text-muted-foreground/40 text-xs shrink-0">
                      {expanded === a.id ? '▲' : '▼'}
                    </span>
                  </div>
                  {expanded === a.id && <AgentDetail agent={a} />}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </AppLayout>
  )
}
