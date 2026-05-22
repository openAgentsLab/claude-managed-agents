import { apiFetch } from './client'

export interface ResourceItem {
  id: string
  type: 'file' | 'git'
  target_path: string
  url?: string    // git only
  branch?: string // git only
  created_at: number
}

export interface OutputEntry {
  path: string
  size: number
}

export async function listResources(sessionId: string): Promise<ResourceItem[]> {
  const res = await apiFetch(`/v1/sessions/${encodeURIComponent(sessionId)}/resources`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function addFileResource(
  sessionId: string,
  file: File,
  targetPath: string,
): Promise<string> {
  const buf = await file.arrayBuffer()
  const b64 = btoa(String.fromCharCode(...new Uint8Array(buf)))
  const res = await apiFetch(`/v1/sessions/${encodeURIComponent(sessionId)}/resources`, {
    method: 'POST',
    body: JSON.stringify({ type: 'file', target_path: targetPath, content_base64: b64 }),
  })
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.resource_id as string
}

export async function addFileUrlResource(
  sessionId: string,
  sourceUrl: string,
  targetPath: string,
): Promise<string> {
  const res = await apiFetch(`/v1/sessions/${encodeURIComponent(sessionId)}/resources`, {
    method: 'POST',
    body: JSON.stringify({ type: 'file', target_path: targetPath, source_url: sourceUrl }),
  })
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.resource_id as string
}

export async function addGitResource(
  sessionId: string,
  url: string,
  targetPath: string,
  branch?: string,
  token?: string,
): Promise<string> {
  const res = await apiFetch(`/v1/sessions/${encodeURIComponent(sessionId)}/resources`, {
    method: 'POST',
    body: JSON.stringify({ type: 'git', url, target_path: targetPath, branch: branch || undefined, token: token || undefined }),
  })
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.resource_id as string
}

export async function removeResource(sessionId: string, resourceId: string): Promise<void> {
  const res = await apiFetch(
    `/v1/sessions/${encodeURIComponent(sessionId)}/resources/${encodeURIComponent(resourceId)}`,
    { method: 'DELETE' },
  )
  if (!res.ok && res.status !== 404) throw new Error(await res.text())
}

export async function listOutputs(sessionId: string): Promise<OutputEntry[]> {
  const res = await apiFetch(`/v1/sessions/${encodeURIComponent(sessionId)}/outputs`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function downloadOutput(sessionId: string, path: string): Promise<Blob> {
  const res = await apiFetch(
    `/v1/sessions/${encodeURIComponent(sessionId)}/outputs/${path.replace(/^\//, '')}`,
  )
  if (!res.ok) throw new Error(await res.text())
  return res.blob()
}
