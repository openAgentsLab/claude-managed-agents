import { apiFetch } from './client'

export interface MemoryStore {
  id: string
  name: string
  description: string
  visibility: string
  write_policy: string
  created_by: string
  created_at: number
}

export interface CreateMemoryStoreRequest {
  name: string
  description?: string
  visibility?: 'private' | 'shared_tenant'
  write_policy?: 'owner_only' | 'members'
}

export interface UpdateMemoryStoreRequest {
  name?: string
  description?: string
  visibility?: string
  write_policy?: string
}

export async function listMemoryStores(): Promise<MemoryStore[]> {
  const res = await apiFetch('/v1/memory-stores')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createMemoryStore(req: CreateMemoryStoreRequest): Promise<MemoryStore> {
  const res = await apiFetch('/v1/memory-stores', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getMemoryStore(id: string): Promise<MemoryStore> {
  const res = await apiFetch(`/v1/memory-stores/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateMemoryStore(id: string, req: UpdateMemoryStoreRequest): Promise<void> {
  const res = await apiFetch(`/v1/memory-stores/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function deleteMemoryStore(id: string): Promise<void> {
  const res = await apiFetch(`/v1/memory-stores/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) throw new Error(await res.text())
}
