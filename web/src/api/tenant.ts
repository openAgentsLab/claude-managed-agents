import { apiFetch, adminFetch } from './client'

export interface ResourceQuota {
  memory_bytes: number
  nano_cpus: number
}

export interface ModelOverride {
  provider?: string
  api_key?: string
  base_url?: string
  model?: string
  by_azure?: boolean
  api_version?: string
}

export interface BrainOverride {
  effort?: string
  thinking?: string
  max_retries?: number
}

export interface TenantSettings {
  permission_mode?: string
  allow_rules: string[]
  deny_rules: string[]
  resource_quota: ResourceQuota
  model?: ModelOverride | null
  brain?: BrainOverride | null
}

export interface TenantInfo {
  id: string
  name: string
  role: string
  settings: TenantSettings
}

export interface UserInfo {
  username: string
  role: string
}

export async function getTenantInfo(): Promise<TenantInfo> {
  const res = await apiFetch('/v1/tenant')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateTenantSettings(patch: Partial<TenantSettings>): Promise<void> {
  const res = await adminFetch('/v1/tenant/settings', {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function listUsers(): Promise<UserInfo[]> {
  const res = await adminFetch('/v1/tenant/users')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateUserRole(username: string, role: string): Promise<void> {
  const res = await adminFetch(`/v1/tenant/users/${encodeURIComponent(username)}`, {
    method: 'PATCH',
    body: JSON.stringify({ role }),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function createUser(username: string, password: string, role?: string): Promise<void> {
  const res = await adminFetch('/v1/tenant/users', {
    method: 'POST',
    body: JSON.stringify({ username, password, role: role ?? 'member' }),
  })
  if (!res.ok) throw new Error(await res.text())
}
