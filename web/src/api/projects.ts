import { apiFetch } from './client'

export interface GitConfigResponse {
  url?: string
  branch?: string
  username?: string
}

export interface RefFile {
  url: string
  path: string
}

export interface Project {
  id: string
  name: string
  description?: string
  git?: GitConfigResponse
  environment_id?: string
  ref_files?: RefFile[]
  env?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface CreateProjectRequest {
  name: string
  description?: string
  git?: {
    url?: string
    branch?: string
    username?: string
    token?: string
  }
  environment_id?: string
  ref_files?: RefFile[]
  env?: Record<string, string>
}

export async function listProjects(): Promise<Project[]> {
  const res = await apiFetch('/v1/projects')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getProject(id: string): Promise<Project> {
  const res = await apiFetch(`/v1/projects/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createProject(req: CreateProjectRequest): Promise<Project> {
  const res = await apiFetch('/v1/projects', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateProject(id: string, req: CreateProjectRequest): Promise<Project> {
  const res = await apiFetch(`/v1/projects/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteProject(id: string): Promise<void> {
  const res = await apiFetch(`/v1/projects/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) throw new Error(await res.text())
}
