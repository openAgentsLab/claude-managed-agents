import { apiFetch, adminFetch } from './client'

export interface MCPServer {
  name: string
  type: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  url?: string
  headers?: Record<string, string>
  disabled: boolean
  updated_at: string
}

export interface UpsertMCPServerRequest {
  name: string
  type: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  url?: string
  headers?: Record<string, string>
  disabled?: boolean
}

// ── tenant-level MCP (all authed users: read; admin: write) ───────────────────

export async function listMCPServers(): Promise<MCPServer[]> {
  const res = await apiFetch('/v1/tenant/mcp/servers')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listTenantMCPServers(): Promise<MCPServer[]> {
  return listMCPServers()
}

export async function upsertTenantMCPServer(req: UpsertMCPServerRequest): Promise<void> {
  const res = await adminFetch(`/v1/tenant/mcp/servers/${encodeURIComponent(req.name)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function deleteTenantMCPServer(name: string): Promise<void> {
  const res = await adminFetch(`/v1/tenant/mcp/servers/${encodeURIComponent(name)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}
