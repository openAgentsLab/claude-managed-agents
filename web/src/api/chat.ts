import { getToken } from './client'

// SSEEvent types exposed to the UI layer (mapped from harness event types).
export type SSEEvent =
  | { type: 'token'; content: string }
  | { type: 'status'; content: string }
  | { type: 'tool_use'; content: string; tool: string; description?: string }
  | { type: 'task_update'; content: string; task_id: string; subject: string; status: string }
  | { type: 'title'; content: string }

// Raw harness event as sent by GET /v1/sessions/:id/events (SSE).
interface HarnessSSEEvent {
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

/** Map a raw harness event to the UI SSEEvent (or null to skip). */
function mapHarnessEvent(ev: HarnessSSEEvent): SSEEvent | null {
  switch (ev.type) {
    case 'agent.message':
      return { type: 'token', content: ev.content ?? '' }
    case 'agent.thinking':
      return { type: 'status', content: ev.content ?? '' }
    case 'agent.tool_use':
      return { type: 'tool_use', content: ev.description ?? '', tool: ev.tool ?? '', description: ev.description }
    case 'title':
      return { type: 'title', content: ev.content ?? '' }
    case 'agent.outcome_evaluation':
      return {
        type: 'task_update',
        content: ev.outcome_result ?? '',
        task_id: String(ev.outcome_iteration ?? 0),
        subject: `iteration ${ev.outcome_iteration ?? 0}/${ev.outcome_max_iterations ?? 0}`,
        status: ev.outcome_result ?? '',
      }
    default:
      return null
  }
}

/**
 * Send a message to the agent and stream back events.
 *
 * Protocol (two-step):
 *  1. POST /run → 202 Accepted (agent starts in background; events buffered in queue)
 *  2. GET  /events with Accept: text/event-stream → SSE stream until session.status_idle
 *
 * Because the server uses a queue-based EventBus, events published before the
 * SSE connection is established are buffered and replayed — including the initial
 * thinking event — so no special client-side workarounds are needed.
 */
export async function* streamRun(
  sessionId: string,
  message: string,
  mode?: 'default' | 'plan',
): AsyncGenerator<SSEEvent> {
  const token = getToken()
  const authHeader = token ? { Authorization: `Bearer ${token}` } : {}

  // Step 1: trigger the run
  const runRes = await fetch(`/api/v1/sessions/${sessionId}/run`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeader },
    body: JSON.stringify({ message, ...(mode ? { mode } : {}) }),
  })
  if (!runRes.ok) {
    const body = await runRes.text()
    if (runRes.status === 409) throw new Error('Session is already running')
    throw new Error(`Run failed (${runRes.status}): ${body}`)
  }
  await runRes.body?.cancel()

  // Step 2: subscribe to the SSE event stream.
  // The server queue replays any events (including thinking) that were published
  // between the POST and this connection — no client-side synthetic events needed.
  const eventsRes = await fetch(`/api/v1/sessions/${sessionId}/events`, {
    headers: { Accept: 'text/event-stream', ...authHeader },
  })
  if (!eventsRes.ok) {
    throw new Error(`Events stream failed (${eventsRes.status}): ${await eventsRes.text()}`)
  }

  const reader = eventsRes.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  let pendingData = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })

    // SSE spec: events separated by blank lines; fields are "key: value\n"
    const lines = buf.split('\n')
    buf = lines.pop() ?? '' // keep incomplete last line

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        pendingData = line.slice(6).trim()
      } else if (line === '' && pendingData) {
        // Blank line = end of one SSE event
        try {
          const ev: HarnessSSEEvent = JSON.parse(pendingData)
          pendingData = ''

          if (ev.type === 'session.status_idle') {
            return
          }
          if (ev.type === 'session.error') {
            throw new Error(ev.content ?? 'Session error')
          }

          const mapped = mapHarnessEvent(ev)
          if (mapped) yield mapped
        } catch (parseErr) {
          // Rethrow real errors; ignore unparseable data lines
          if (parseErr instanceof SyntaxError) {
            pendingData = ''
          } else {
            throw parseErr
          }
        }
      }
    }
  }
}
