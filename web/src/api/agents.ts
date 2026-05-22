import { apiFetch, adminFetch } from './client'

export interface AgentResponse {
  id: string
  name: string
  description: string
  version: number
  model?: string
  system_prompt?: string
  tool_config?: Record<string, boolean>
  mcp_server_names?: string[]
  skill_names?: string[]
  callable_agents?: string[]
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface CreateAgentRequest {
  name: string
  description?: string
  model?: string
  system_prompt?: string
  tool_config?: Record<string, boolean>
  mcp_server_names?: string[]
  skill_names?: string[]
  callable_agents?: string[]
  is_default?: boolean
}

export interface UpdateAgentRequest {
  name?: string
  description?: string
  model?: string
  system_prompt?: string
  tool_config?: Record<string, boolean>
  mcp_server_names?: string[]
  skill_names?: string[]
  callable_agents?: string[]
  is_default?: boolean
}

// ── User-facing (read) ────────────────────────────────────────────────────────

export async function listAgents(): Promise<AgentResponse[]> {
  const res = await apiFetch('/v1/agents')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getAgent(id: string): Promise<AgentResponse> {
  const res = await apiFetch(`/v1/agents/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

// ── Admin (write) ─────────────────────────────────────────────────────────────

export async function createAgent(req: CreateAgentRequest): Promise<AgentResponse> {
  const res = await adminFetch('/v1/agents', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateAgent(id: string, req: UpdateAgentRequest): Promise<AgentResponse> {
  const res = await adminFetch(`/v1/agents/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function archiveAgent(id: string): Promise<void> {
  const res = await adminFetch(`/v1/agents/${encodeURIComponent(id)}/archive`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}
