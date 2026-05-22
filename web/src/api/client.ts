export function getToken(): string | null {
  return localStorage.getItem('forge_token')
}

function buildHeaders(init: RequestInit): Record<string, string> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init.headers as Record<string, string> | undefined),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`
  return headers
}

function handle401(res: Response): Response {
  if (res.status === 401) {
    localStorage.removeItem('forge_token')
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }
  return res
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const res = await fetch(`/api${path}`, { ...init, headers: buildHeaders(init) })
  return handle401(res)
}

export async function adminFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const res = await fetch(`/admin${path}`, { ...init, headers: buildHeaders(init) })
  return handle401(res)
}
