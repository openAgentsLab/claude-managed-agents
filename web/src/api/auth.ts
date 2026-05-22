import { apiFetch } from './client'

export async function login(username: string, password: string): Promise<string> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text.trim() || 'Login failed')
  }
  const data = await res.json()
  return data.token as string
}

export async function logout(): Promise<void> {
  await apiFetch('/auth/logout', { method: 'POST' })
}
