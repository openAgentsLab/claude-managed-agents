import { apiFetch, adminFetch } from './client'

export interface SkillMeta {
  name: string
  updated_at: string
}

export interface SkillFull extends SkillMeta {
  content: string
}

export interface UpsertSkillRequest {
  name: string
  content: string
}

// ── tenant-level skills (all authed users: read; admin: write) ─────────────────

export async function listSkills(): Promise<SkillMeta[]> {
  const res = await apiFetch('/v1/tenant/skills')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getSkill(name: string): Promise<SkillFull> {
  const res = await apiFetch(`/v1/tenant/skills/${encodeURIComponent(name)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listTenantSkills(): Promise<SkillMeta[]> {
  return listSkills()
}

export async function getTenantSkill(name: string): Promise<SkillFull> {
  return getSkill(name)
}

export async function upsertTenantSkill(req: UpsertSkillRequest): Promise<void> {
  const res = await adminFetch(`/v1/tenant/skills/${encodeURIComponent(req.name)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function deleteTenantSkill(name: string): Promise<void> {
  const res = await adminFetch(`/v1/tenant/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}
