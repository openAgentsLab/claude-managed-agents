import { useState, useEffect, type FormEvent } from 'react'
import AppLayout from '@/components/AppLayout'
import {
  listMemoryStores,
  createMemoryStore,
  updateMemoryStore,
  deleteMemoryStore,
  type MemoryStore,
} from '@/api/memory'
import { useAuth } from '@/App'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

function StoreForm({
  initial,
  saving,
  onSave,
  onCancel,
}: {
  initial?: MemoryStore
  saving: boolean
  onSave: (name: string, description: string, visibility: string, writePolicy: string) => void
  onCancel: () => void
}) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [visibility, setVisibility] = useState(initial?.visibility ?? 'private')
  const [writePolicy, setWritePolicy] = useState(initial?.write_policy ?? 'owner_only')

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    onSave(name.trim(), description, visibility, writePolicy)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Name *</Label>
          <Input className="h-7 text-sm" value={name} onChange={(e) => setName(e.target.value)} placeholder="my-memory" required />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Description</Label>
          <Input className="h-7 text-sm" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What this store is for" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Visibility</Label>
          <select
            className="h-7 w-full rounded-md border border-input bg-background px-3 text-sm"
            value={visibility}
            onChange={(e) => setVisibility(e.target.value)}
          >
            <option value="private">Private (only me)</option>
            <option value="shared_tenant">Shared (all tenant members)</option>
          </select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Write Policy</Label>
          <select
            className="h-7 w-full rounded-md border border-input bg-background px-3 text-sm"
            value={writePolicy}
            onChange={(e) => setWritePolicy(e.target.value)}
          >
            <option value="owner_only">Owner only</option>
            <option value="members">All members (when shared)</option>
          </select>
        </div>
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim()}>
          {saving ? 'Saving…' : initial ? 'Update' : 'Create Store'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

export default function MemoryStoresPage() {
  const { username, tenantId } = useAuth()
  const myUserID = tenantId && username ? `${tenantId}/${username}` : ''

  const [stores, setStores] = useState<MemoryStore[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)

  useEffect(() => {
    listMemoryStores()
      .then(setStores)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleCreate(name: string, description: string, visibility: string, writePolicy: string) {
    setSaving(true)
    setError('')
    try {
      const s = await createMemoryStore({
        name,
        description,
        visibility: visibility as 'private' | 'shared_tenant',
        write_policy: writePolicy as 'owner_only' | 'members',
      })
      setStores((prev) => [s, ...prev])
      setShowCreate(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(id: string, name: string, description: string, visibility: string, writePolicy: string) {
    setSaving(true)
    setError('')
    try {
      await updateMemoryStore(id, { name, description, visibility, write_policy: writePolicy })
      const updated = await listMemoryStores()
      setStores(updated)
      setEditId(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete memory store "${name}"?`)) return
    setError('')
    try {
      await deleteMemoryStore(id)
      setStores((prev) => prev.filter((s) => s.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const visibilityLabel: Record<string, string> = {
    private: 'Private',
    shared_tenant: 'Shared',
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-5">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Memory Stores</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Custom memory stores that can be mounted into sessions.
            </p>
          </div>
          {!showCreate && (
            <Button size="sm" onClick={() => { setShowCreate(true); setEditId(null) }}>
              + New Store
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showCreate && (
          <StoreForm saving={saving} onSave={handleCreate} onCancel={() => setShowCreate(false)} />
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : stores.length === 0 && !showCreate ? (
          <div className="text-center py-16 text-sm text-muted-foreground">No memory stores yet</div>
        ) : (
          <div className="space-y-2">
            {stores.map((s) => {
              const isOwner = s.created_by === myUserID
              return (
                <Card key={s.id}>
                  <CardContent className="px-4 py-3">
                    {editId === s.id ? (
                      <StoreForm
                        initial={s}
                        saving={saving}
                        onSave={(name, desc, vis, wp) => handleUpdate(s.id, name, desc, vis, wp)}
                        onCancel={() => setEditId(null)}
                      />
                    ) : (
                      <div className="flex items-start gap-3">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <p className="text-sm font-medium text-foreground">{s.name}</p>
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${
                              s.visibility === 'shared_tenant'
                                ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400'
                                : 'bg-muted text-muted-foreground'
                            }`}>
                              {visibilityLabel[s.visibility] ?? s.visibility}
                            </span>
                            {!isOwner && (
                              <span className="text-[10px] text-muted-foreground">read-only</span>
                            )}
                          </div>
                          {s.description && (
                            <p className="text-xs text-muted-foreground mt-0.5">{s.description}</p>
                          )}
                          <p className="text-[11px] text-muted-foreground/60 mt-1">
                            ID: <span className="font-mono">{s.id}</span>
                            {!isOwner && ` · by ${s.created_by}`}
                          </p>
                        </div>
                        {isOwner && (
                          <div className="flex gap-1 shrink-0">
                            <Button
                              variant="ghost"
                              size="xs"
                              onClick={() => { setEditId(s.id); setShowCreate(false) }}
                              className="text-muted-foreground hover:text-foreground"
                            >
                              Edit
                            </Button>
                            <Button
                              variant="ghost"
                              size="xs"
                              onClick={() => handleDelete(s.id, s.name)}
                              className="text-muted-foreground hover:text-destructive"
                            >
                              Delete
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </div>
    </AppLayout>
  )
}
