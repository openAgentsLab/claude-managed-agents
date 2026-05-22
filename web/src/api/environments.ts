import { apiFetch, adminFetch } from './client'

export interface PackageList {
  pip?: string[]
  npm?: string[]
  apt?: string[]
  cargo?: string[]
}

export interface NetworkingConfig {
  mode: string
  allowed_hosts?: string[]
}

export interface Environment {
  id: string
  scope: string
  name: string
  description?: string
  packages?: PackageList
  networking: NetworkingConfig
  env?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface EnvironmentRequest {
  name: string
  description?: string
  packages?: PackageList
  networking?: {
    mode: string
    allowed_hosts?: string[]
  }
  env?: Record<string, string>
}

// ── User-scoped (read-only) ───────────────────────────────────────────────────

export async function listEnvironments(): Promise<Environment[]> {
  const res = await apiFetch('/v1/environments')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

// ── Admin tenant-scoped ───────────────────────────────────────────────────────

export async function listTenantEnvironments(): Promise<Environment[]> {
  const res = await adminFetch('/v1/environments')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createTenantEnvironment(req: EnvironmentRequest): Promise<Environment> {
  const res = await adminFetch('/v1/environments', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateTenantEnvironment(id: string, req: EnvironmentRequest): Promise<Environment> {
  const res = await adminFetch(`/v1/environments/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteTenantEnvironment(id: string): Promise<void> {
  const res = await adminFetch(`/v1/environments/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) throw new Error(await res.text())
}
