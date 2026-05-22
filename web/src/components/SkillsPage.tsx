import { useState, useEffect, type FormEvent } from 'react'
import { type SkillMeta, type SkillFull, type UpsertSkillRequest } from '@/api/skills'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'

const SKILL_TEMPLATE = `---
name: my-skill
description: What this skill does
---

# Instructions

Describe the skill here. The agent will read this when the skill is invoked.
`

interface SkillFormProps {
  initial?: { name: string; content: string }
  isNew?: boolean
  saving: boolean
  onSave: (req: UpsertSkillRequest) => void
  onCancel: () => void
}

function SkillForm({ initial, isNew, saving, onSave, onCancel }: SkillFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [content, setContent] = useState(initial?.content ?? SKILL_TEMPLATE)

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim() || !content.trim()) return
    onSave({ name: name.trim(), content })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 bg-muted/30 rounded-lg border border-border">
      <div className="space-y-1.5">
        <Label className="text-xs">Name *</Label>
        <Input
          className="h-7 text-sm"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="my-skill"
          disabled={!isNew}
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">Content (SKILL.md format)</Label>
        <Textarea
          className="font-mono text-xs min-h-[240px] resize-y"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={SKILL_TEMPLATE}
          required
        />
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !name.trim()}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

export interface SkillsPageProps {
  title: string
  description?: string
  listFn: () => Promise<SkillMeta[]>
  getFn: (name: string) => Promise<SkillFull>
  upsertFn?: (req: UpsertSkillRequest) => Promise<void>
  deleteFn?: (name: string) => Promise<void>
  readOnly?: boolean
}

export default function SkillsPage({ title, description, listFn, getFn, upsertFn, deleteFn, readOnly }: SkillsPageProps) {
  const [skills, setSkills] = useState<SkillMeta[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)
  const [editingFull, setEditingFull] = useState<SkillFull | null>(null)
  const [loadingEdit, setLoadingEdit] = useState(false)

  useEffect(() => {
    listFn()
      .then(setSkills)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleExpand(name: string) {
    if (editing === name) {
      setEditing(null)
      setEditingFull(null)
      return
    }
    setEditing(name)
    setLoadingEdit(true)
    try {
      const full = await getFn(name)
      setEditingFull(full)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load skill')
    } finally {
      setLoadingEdit(false)
    }
  }

  async function handleSave(req: UpsertSkillRequest) {
    if (!upsertFn) return
    setSaving(true)
    setError('')
    try {
      await upsertFn(req)
      const list = await listFn()
      setSkills(list)
      setShowForm(false)
      setEditing(null)
      setEditingFull(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(name: string) {
    if (!deleteFn) return
    if (!confirm(`Delete skill "${name}"?`)) return
    setError('')
    try {
      await deleteFn(name)
      setSkills((s) => s.filter((x) => x.name !== name))
      if (editing === name) {
        setEditing(null)
        setEditingFull(null)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-5">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-lg font-semibold text-foreground">{title}</h1>
          {description && <p className="text-sm text-muted-foreground mt-0.5">{description}</p>}
        </div>
        {!readOnly && !showForm && (
          <Button size="sm" onClick={() => setShowForm(true)}>
            + Add Skill
          </Button>
        )}
      </div>

      {error && (
        <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
          {error}
        </div>
      )}

      {showForm && (
        <SkillForm
          isNew
          saving={saving}
          onSave={handleSave}
          onCancel={() => setShowForm(false)}
        />
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading…</div>
      ) : skills.length === 0 ? (
        <div className="text-center py-16 text-sm text-muted-foreground">No skills configured</div>
      ) : (
        <div className="space-y-2">
          {skills.map((s) => (
            <Card key={s.name} className="overflow-hidden">
              <CardContent className="p-0">
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors"
                  onClick={() => handleExpand(s.name)}
                >
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-medium text-foreground">{s.name}</span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {!readOnly && deleteFn && (
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={(e) => { e.stopPropagation(); handleDelete(s.name) }}
                        className="text-muted-foreground hover:text-destructive"
                      >
                        Delete
                      </Button>
                    )}
                    <span className="text-muted-foreground/40 text-xs">{editing === s.name ? '▲' : '▼'}</span>
                  </div>
                </div>
                {editing === s.name && (
                  <div className="border-t border-border">
                    {loadingEdit ? (
                      <div className="p-4 text-sm text-muted-foreground">Loading…</div>
                    ) : editingFull ? (
                      readOnly ? (
                        <Textarea
                          className="font-mono text-xs min-h-[200px] resize-y border-0 rounded-none bg-muted/20 focus-visible:ring-0 cursor-default"
                          value={editingFull.content}
                          readOnly
                        />
                      ) : (
                        <SkillForm
                          initial={editingFull}
                          saving={saving}
                          onSave={handleSave}
                          onCancel={() => { setEditing(null); setEditingFull(null) }}
                        />
                      )
                    ) : null}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
