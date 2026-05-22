import { apiFetch } from './client'
import type { ModelOverride, BrainOverride } from './tenant'

export type { ModelOverride, BrainOverride }

export interface UserSettings {
  model?: ModelOverride | null
  brain?: BrainOverride | null
}

export async function getUserSettings(): Promise<UserSettings> {
  const res = await apiFetch('/v1/user/settings')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateUserSettings(patch: UserSettings): Promise<void> {
  const res = await apiFetch('/v1/user/settings', {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
  if (!res.ok) throw new Error(await res.text())
}
