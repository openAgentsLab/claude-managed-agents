import { useState, useEffect } from 'react'
import AppLayout from '@/components/AppLayout'
import { listEnvironments, type Environment } from '@/api/environments'
import { Card, CardContent } from '@/components/ui/card'

// ── env card ──────────────────────────────────────────────────────────────────

function EnvCard({ env }: { env: Environment }) {
  const pkgCounts = [
    env.packages?.pip?.length,
    env.packages?.npm?.length,
    env.packages?.apt?.length,
    env.packages?.cargo?.length,
  ].reduce((a, b) => (a ?? 0) + (b ?? 0), 0) ?? 0

  return (
    <Card>
      <CardContent className="px-4 py-3">
        <div className="flex items-start gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium text-foreground">{env.name}</p>
              <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                {env.scope}
              </span>
            </div>
            {env.description && (
              <p className="text-xs text-muted-foreground mt-0.5">{env.description}</p>
            )}
            <div className="flex flex-wrap gap-3 mt-1">
              {env.networking?.mode === 'limited' && (
                <span className="text-[11px] text-muted-foreground">network: limited</span>
              )}
              {pkgCounts > 0 && (
                <span className="text-[11px] text-muted-foreground">
                  {pkgCounts} package{pkgCounts !== 1 ? 's' : ''}
                </span>
              )}
              {env.env && Object.keys(env.env).length > 0 && (
                <span className="text-[11px] text-muted-foreground">
                  {Object.keys(env.env).length} env var{Object.keys(env.env).length !== 1 ? 's' : ''}
                </span>
              )}
            </div>
            <p className="text-[11px] text-muted-foreground/60 mt-1 font-mono">id: {env.id}</p>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ── page ──────────────────────────────────────────────────────────────────────

export default function EnvironmentsPage() {
  const [envs, setEnvs] = useState<Environment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    listEnvironments()
      .then(setEnvs)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  return (
    <AppLayout>
      <div className="p-6 max-w-2xl mx-auto space-y-6">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Environments</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Sandbox environments configured by admins. Reference an environment by its ID in project settings.
          </p>
        </div>

        {error && (
          <div className="text-destructive text-sm bg-destructive/10 border border-destructive/30 rounded-md px-4 py-3">
            {error}
          </div>
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading…</div>
        ) : envs.length === 0 ? (
          <div className="text-center py-16 text-sm text-muted-foreground">
            No environments configured. Ask your admin to add one.
          </div>
        ) : (
          <div className="space-y-2">
            {envs.map((env) => (
              <EnvCard key={env.id} env={env} />
            ))}
          </div>
        )}
      </div>
    </AppLayout>
  )
}
