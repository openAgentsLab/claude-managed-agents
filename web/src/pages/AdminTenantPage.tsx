import { useState, useEffect } from 'react'
import { getTenantInfo, updateTenantSettings, type TenantSettings } from '@/api/tenant'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import AppLayout from '@/components/AppLayout'
import ModelBrainConfig from '@/components/ModelBrainConfig'

const PERMISSION_MODES = ['default', 'plan']

export default function AdminTenantPage() {
  const [settings, setSettings] = useState<TenantSettings>({
    permission_mode: 'default',
    allow_rules: [],
    deny_rules: [],
    resource_quota: { memory_bytes: 0, nano_cpus: 0 },
  })
  const [tenantName, setTenantName] = useState('')
  const [newAllowRule, setNewAllowRule] = useState('')
  const [newDenyRule, setNewDenyRule] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    getTenantInfo()
      .then((info) => {
        setTenantName(info.name || info.id)
        setSettings(info.settings)
      })
      .catch((err) => setError(err.message))
  }, [])

  async function handleSave() {
    setSaving(true)
    setSaved(false)
    setError('')
    try {
      await updateTenantSettings(settings)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  function addAllowRule() {
    const rule = newAllowRule.trim()
    if (!rule) return
    setSettings((s) => ({ ...s, allow_rules: [...(s.allow_rules ?? []), rule] }))
    setNewAllowRule('')
  }

  function removeAllowRule(i: number) {
    setSettings((s) => ({ ...s, allow_rules: s.allow_rules?.filter((_, idx) => idx !== i) ?? [] }))
  }

  function addDenyRule() {
    const rule = newDenyRule.trim()
    if (!rule) return
    setSettings((s) => ({ ...s, deny_rules: [...(s.deny_rules ?? []), rule] }))
    setNewDenyRule('')
  }

  function removeDenyRule(i: number) {
    setSettings((s) => ({ ...s, deny_rules: s.deny_rules?.filter((_, idx) => idx !== i) ?? [] }))
  }

  const memoryMB = settings.resource_quota
    ? Math.round(settings.resource_quota.memory_bytes / (1024 * 1024))
    : 0
  const cpuCores = settings.resource_quota
    ? settings.resource_quota.nano_cpus / 1_000_000_000
    : 0

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-6">
        <div>
          <h1 className="text-lg font-semibold text-foreground">
            {tenantName ? `${tenantName} — Settings` : 'Tenant Settings'}
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">Permission policy and resource limits for this tenant.</p>
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {/* Permission Mode */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Permission Mode</CardTitle>
          </CardHeader>
          <CardContent>
            <select
              value={settings.permission_mode || 'default'}
              onChange={(e) => setSettings((s) => ({ ...s, permission_mode: e.target.value }))}
              className="w-full h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              {PERMISSION_MODES.map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
          </CardContent>
        </Card>

        {/* Allow Rules */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Allow Rules</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              {(settings.allow_rules ?? []).map((rule, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Badge variant="outline" className="font-mono text-xs flex-1 justify-start">{rule}</Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeAllowRule(i)}
                    className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                  >
                    ×
                  </Button>
                </div>
              ))}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="e.g. Bash(git:*)"
                value={newAllowRule}
                onChange={(e) => setNewAllowRule(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && addAllowRule()}
                className="font-mono text-xs"
              />
              <Button variant="outline" size="sm" onClick={addAllowRule}>Add</Button>
            </div>
          </CardContent>
        </Card>

        {/* Deny Rules */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Deny Rules</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              {(settings.deny_rules ?? []).map((rule, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Badge variant="outline" className="font-mono text-xs flex-1 justify-start text-destructive border-destructive/30">{rule}</Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeDenyRule(i)}
                    className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                  >
                    ×
                  </Button>
                </div>
              ))}
            </div>
            <div className="flex gap-2">
              <Input
                placeholder="e.g. Bash(rm:*)"
                value={newDenyRule}
                onChange={(e) => setNewDenyRule(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && addDenyRule()}
                className="font-mono text-xs"
              />
              <Button variant="outline" size="sm" onClick={addDenyRule}>Add</Button>
            </div>
          </CardContent>
        </Card>

        {/* Resource Quota */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Resource Quota</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="memory">Memory (MB)</Label>
              <Input
                id="memory"
                type="number"
                min="0"
                value={memoryMB || ''}
                placeholder="0 = unlimited"
                onChange={(e) => {
                  const mb = parseInt(e.target.value) || 0
                  setSettings((s) => ({
                    ...s,
                    resource_quota: { ...s.resource_quota, memory_bytes: mb * 1024 * 1024 },
                  }))
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="cpu">CPU Cores</Label>
              <Input
                id="cpu"
                type="number"
                min="0"
                step="0.5"
                value={cpuCores || ''}
                placeholder="0 = unlimited"
                onChange={(e) => {
                  const cores = parseFloat(e.target.value) || 0
                  setSettings((s) => ({
                    ...s,
                    resource_quota: {
                      ...s.resource_quota,
                      nano_cpus: Math.round(cores * 1_000_000_000),
                    },
                  }))
                }}
              />
            </div>
          </CardContent>
        </Card>

        {/* Model / Brain Config */}
        <ModelBrainConfig
          context="tenant"
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
