import { useState, useEffect } from 'react'
import { getUserSettings, updateUserSettings, type UserSettings } from '@/api/userSettings'
import { Button } from '@/components/ui/button'
import AppLayout from '@/components/AppLayout'
import ModelBrainConfig from '@/components/ModelBrainConfig'

export default function UserModelPage() {
  const [settings, setSettings] = useState<UserSettings>({})
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    getUserSettings()
      .then(setSettings)
      .catch((err) => setError(err.message))
  }, [])

  async function handleSave() {
    setSaving(true)
    setSaved(false)
    setError('')
    try {
      await updateUserSettings(settings)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-6">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Model Settings</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Override the model and inference settings for your account. Leave overrides disabled to inherit from tenant defaults.
          </p>
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        <ModelBrainConfig
          model={settings.model}
          brain={settings.brain}
          onModelChange={(m) => setSettings((s) => ({ ...s, model: m }))}
          onBrainChange={(b) => setSettings((s) => ({ ...s, brain: b }))}
        />

        <div className="flex items-center gap-3">
          <Button onClick={handleSave} disabled={saving}>
            {saving ? 'Saving…' : 'Save Changes'}
          </Button>
          {saved && <span className="text-sm text-muted-foreground">Saved</span>}
        </div>
      </div>
    </AppLayout>
  )
}
