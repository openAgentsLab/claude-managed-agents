import { useState, useEffect, type FormEvent } from 'react'
import { listUsers, updateUserRole, createUser, type UserInfo } from '@/api/tenant'
import { useAuth } from '@/App'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import AppLayout from '@/components/AppLayout'

const ROLES = ['admin', 'member', 'viewer']

function roleBadgeVariant(role: string): 'default' | 'secondary' | 'outline' {
  if (role === 'admin') return 'default'
  if (role === 'viewer') return 'secondary'
  return 'outline'
}

function CreateUserForm({
  saving,
  onSave,
  onCancel,
}: {
  saving: boolean
  onSave: (username: string, password: string, role: string) => void
  onCancel: () => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('member')

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!username.trim() || !password) return
    onSave(username.trim(), password, role)
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-3 p-4 bg-muted/30 rounded-lg border border-border"
    >
      <div className="grid grid-cols-3 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Username *</Label>
          <Input
            className="h-7 text-sm"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="alice"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Password *</Label>
          <Input
            className="h-7 text-sm"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Role</Label>
          <select
            className="h-7 w-full rounded-md border border-input bg-background px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            value={role}
            onChange={(e) => setRole(e.target.value)}
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
        </div>
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !username.trim() || !password}>
          {saving ? 'Creating…' : 'Create User'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

export default function AdminUsersPage() {
  const { username } = useAuth()
  const [users, setUsers] = useState<UserInfo[]>([])
  const [updating, setUpdating] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listUsers()
      .then(setUsers)
      .catch((err) => setError(err.message))
  }, [])

  async function handleRoleChange(user: string, role: string) {
    setUpdating(user)
    setError('')
    try {
      await updateUserRole(user, role)
      setUsers((prev) => prev.map((u) => (u.username === user ? { ...u, role } : u)))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role')
    } finally {
      setUpdating(null)
    }
  }

  async function handleCreateUser(uname: string, password: string, role: string) {
    setCreating(true)
    setError('')
    try {
      await createUser(uname, password, role)
      const list = await listUsers()
      setUsers(list)
      setShowCreateForm(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create user')
    } finally {
      setCreating(false)
    }
  }

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-5">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">Users</h1>
            <p className="text-sm text-muted-foreground mt-0.5">Manage user accounts and roles in this tenant.</p>
          </div>
          {!showCreateForm && (
            <Button size="sm" onClick={() => setShowCreateForm(true)}>
              + Add User
            </Button>
          )}
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {showCreateForm && (
          <CreateUserForm
            saving={creating}
            onSave={handleCreateUser}
            onCancel={() => setShowCreateForm(false)}
          />
        )}

        <div className="space-y-2">
          {users.map((u) => (
            <Card key={u.username}>
              <CardContent className="px-5 py-4 flex items-center gap-4">
                <div className="flex-1 flex items-center gap-3">
                  <span className="text-sm font-medium text-foreground">{u.username}</span>
                  {u.username === username && (
                    <span className="text-xs text-muted-foreground">(you)</span>
                  )}
                  <Badge variant={roleBadgeVariant(u.role)} className="text-xs capitalize">
                    {u.role}
                  </Badge>
                </div>
                <select
                  value={u.role}
                  disabled={u.username === username || updating === u.username}
                  onChange={(e) => handleRoleChange(u.username, e.target.value)}
                  className="h-8 rounded-md border border-input bg-background px-2 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {ROLES.map((r) => (
                    <option key={r} value={r}>{r}</option>
                  ))}
                </select>
              </CardContent>
            </Card>
          ))}
        </div>

        {users.length > 0 && (
          <p className="text-xs text-muted-foreground text-center">
            角色变更在用户下次登录后生效
          </p>
        )}
      </div>
    </AppLayout>
  )
}
