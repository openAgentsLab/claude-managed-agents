import { apiFetch } from './client'

export interface SessionInfo {
  session_id: string
  project_id?: string
  project_name?: string
  environment_id?: string
  status: string
}

/** A single harness event as returned by GET /v1/sessions/:id/events/history. */
export interface HarnessEvent {
  type: string
  content?: string
  tool?: string
  tool_use_id?: string
  tool_input?: string
  description?: string
  stop_reason?: string
  outcome_result?: string
  outcome_iteration?: number
  outcome_max_iterations?: number
  seq?: number
}

export interface SessionListItem {
  session_id: string
  project_id?: string
  title?: string
  status: string
  init_error?: string
  created_at: string
}

/** Metadata returned by GET /v1/sessions/:id. */
export interface SessionMeta {
  session_id: string
  project_id?: string
  title?: string
  status: string
  init_error?: string
  created_at: string
}

export interface CreateSessionRequest {
  session_id?: string
  project_id?: string
  agent_id?: string
  memory_stores?: string[]
}

export async function listSessions(): Promise<SessionListItem[]> {
  const res = await apiFetch('/v1/sessions')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createSession(req?: CreateSessionRequest): Promise<SessionInfo> {
  const res = await apiFetch('/v1/sessions', {
    method: 'POST',
    body: JSON.stringify(req ?? {}),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

/** Poll a single session's status (GET /v1/sessions/:id). */
export async function getSession(sessionId: string): Promise<SessionMeta> {
  const res = await apiFetch(`/v1/sessions/${sessionId}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

/** Fetch full conversation history as harness events (GET /v1/sessions/:id/events/history). */
export async function getSessionHistory(sessionId: string, signal?: AbortSignal): Promise<HarnessEvent[]> {
  const res = await apiFetch(`/v1/sessions/${sessionId}/events/history`, { signal })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteSession(sessionId: string): Promise<void> {
  const res = await apiFetch(`/v1/sessions/${sessionId}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) throw new Error(await res.text())
}

export async function clearHistory(sessionId: string): Promise<void> {
  const res = await apiFetch(`/v1/sessions/${sessionId}/clear`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}

export async function updateSessionTitle(sessionId: string, title: string): Promise<void> {
  const res = await apiFetch(`/v1/sessions/${sessionId}`, {
    method: 'PATCH',
    body: JSON.stringify({ title }),
  })
  if (!res.ok) throw new Error(await res.text())
}
