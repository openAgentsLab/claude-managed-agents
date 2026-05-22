import { apiFetch, adminFetch } from './client'

export interface VaultItem {
  name: string
  description: string
  updated_at: string
}

export async function listVaults(): Promise<VaultItem[]> {
  const res = await apiFetch('/v1/vaults')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function setVault(name: string, value: string, description?: string): Promise<void> {
  const res = await apiFetch('/v1/vaults', {
    method: 'POST',
    body: JSON.stringify({ name, value, description: description ?? '' }),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function deleteVault(name: string): Promise<void> {
  const res = await apiFetch(`/v1/vaults/${encodeURIComponent(name)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}

export async function listTenantVaults(): Promise<VaultItem[]> {
  const res = await adminFetch('/v1/tenant/vaults')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function setTenantVault(name: string, value: string, description?: string): Promise<void> {
  const res = await adminFetch('/v1/tenant/vaults', {
    method: 'POST',
    body: JSON.stringify({ name, value, description: description ?? '' }),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function deleteTenantVault(name: string): Promise<void> {
  const res = await adminFetch(`/v1/tenant/vaults/${encodeURIComponent(name)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}
