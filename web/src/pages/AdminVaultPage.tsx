import { useState, useEffect, type FormEvent } from 'react'
import AppLayout from '@/components/AppLayout'
import { listTenantVaults, setTenantVault, deleteTenantVault, type VaultItem } from '@/api/vault'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

function AddVaultForm({
  saving,
  onSave,
  onCancel,
}: {
  saving: boolean
  onSave: (name: string, value: string, description: string) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [value, setValue] = useState('')
  const [description, setDescription] = useState('')

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim() || !value.trim()) return
    onSave(name.trim(), value, description)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input
            className="h-7 text-sm font-mono"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="GITHUB_TOKEN"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Description</Label>
          <Input
            className="h-7 text-sm"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="GitHub personal access token"
          />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs">Value *</Label>
        <Input
          className="h-7 text-sm font-mono"
          type="password"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="ghp_…"
          required
        />
        <p className="text-[11px] text-muted-foreground">
          Encrypted with AES-256-GCM. The value is never returned by the API.
          Reference it in MCP env with <code className="font-mono">vault:NAME</code>.
        </p>
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim() || !value.trim()}>
          {saving ? 'Saving…' : 'Save Secret'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

export default function AdminVaultPage() {
  const [items, setItems] = useState<VaultItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showForm, setShowForm] = useState(false)

  useEffect(() => {
    listTenantVaults()
      .then(setItems)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleSave(name: string, value: string, description: string) {
    setSaving(true)
    setError('')
    try {
      await setTenantVault(name, value, description)
      const list = await listTenantVaults()
      setItems(list)
      setShowForm(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(name: string) {
    if (!confirm(`Delete tenant secret "${name}"?`)) return
    setError('')
    try {
      await deleteTenantVault(name)
      setItems((s) => s.filter((x) => x.name !== name))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-5">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Tenant Vault</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Shared encrypted secrets available to all users in this tenant. Reference with <code className="font-mono text-xs">vault:NAME</code>.
            </p>
          </div>
          {!showForm && (
            <Button size="sm" onClick={() => setShowForm(true)}>
              + Add Secret
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showForm && (
          <AddVaultForm saving={saving} onSave={handleSave} onCancel={() => setShowForm(false)} />
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : items.length === 0 ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No tenant secrets stored</div>
        ) : (
          <div className="space-y-2">
            {items.map((item) => (
              <Card key={item.name}>
                <CardContent className="px-4 py-3 flex items-center gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium font-mono text-foreground">{item.name}</p>
                    {item.description && (
                      <p className="text-xs text-muted-foreground mt-0.5">{item.description}</p>
                    )}
                    <p className="text-[11px] text-muted-foreground/60 mt-0.5">
                      Updated {new Date(item.updated_at).toLocaleString()}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    size="xs"
                    onClick={() => handleDelete(item.name)}
                    className="text-muted-foreground hover:text-destructive shrink-0"
                  >
                    Delete
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </AppLayout>
  )
}
