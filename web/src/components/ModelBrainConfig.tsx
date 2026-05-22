import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { ModelOverride, BrainOverride } from '@/api/tenant'

const EFFORT_OPTIONS = ['', 'max', 'high', 'medium', 'low']
const THINKING_OPTIONS = ['', 'adaptive', 'disabled']
const PROVIDER_OPTIONS = ['', 'anthropic', 'openai']

interface Props {
  model: ModelOverride | null | undefined
  brain: BrainOverride | null | undefined
  onModelChange: (m: ModelOverride | null) => void
  onBrainChange: (b: BrainOverride | null) => void
  readOnly?: boolean
  context?: 'tenant' | 'user'
}

function field(label: string, id: string, children: React.ReactNode) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="text-xs">{label}</Label>
      {children}
    </div>
  )
}

export default function ModelBrainConfig({ model, brain, onModelChange, onBrainChange, readOnly, context = 'user' }: Props) {
  const m = model ?? {}
  const b = brain ?? {}
  const modelEnabled = model != null
  const brainEnabled = brain != null
  const toggleLabel = context === 'tenant' ? 'Configure' : 'Override'
  const inheritMsg = context === 'tenant' ? 'No custom config set.' : 'Inheriting from tenant settings.'

  function setModel(patch: Partial<ModelOverride>) {
    onModelChange({ ...m, ...patch })
  }

  function setBrain(patch: Partial<BrainOverride>) {
    onBrainChange({ ...b, ...patch })
  }

  const inputCls = 'h-8 text-sm'
  const selectCls = 'w-full h-8 rounded-md border border-input bg-background px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50'

  return (
    <div className="space-y-4">
      {/* Model Config */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Model Config</CardTitle>
            {!readOnly && (
              <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <input
                  type="checkbox"
                  checked={modelEnabled}
                  onChange={(e) => onModelChange(e.target.checked ? {} : null)}
                  className="rounded"
                />
                {toggleLabel}
              </label>
            )}
          </div>
          {!modelEnabled && (
            <p className="text-xs text-muted-foreground">{inheritMsg}</p>
          )}
        </CardHeader>
        {modelEnabled && (
          <CardContent className="grid grid-cols-2 gap-3">
            {field('Provider', 'model-provider',
              <select
                id="model-provider"
                value={m.provider ?? ''}
                onChange={(e) => setModel({ provider: e.target.value || undefined })}
                disabled={readOnly}
                className={selectCls}
              >
                {PROVIDER_OPTIONS.map((o) => (
                  <option key={o} value={o}>{o || '(inherit)'}</option>
                ))}
              </select>
            )}
            {field('Model', 'model-name',
              <Input
                id="model-name"
                value={m.model ?? ''}
                onChange={(e) => setModel({ model: e.target.value || undefined })}
                disabled={readOnly}
                placeholder="e.g. claude-sonnet-4-6"
                className={inputCls}
              />
            )}
            {field('API Key', 'model-apikey',
              <Input
                id="model-apikey"
                type="password"
                value={m.api_key ?? ''}
                onChange={(e) => setModel({ api_key: e.target.value || undefined })}
                disabled={readOnly}
                placeholder="(inherit)"
                className={inputCls}
              />
            )}
            {field('Base URL', 'model-baseurl',
              <Input
                id="model-baseurl"
                value={m.base_url ?? ''}
                onChange={(e) => setModel({ base_url: e.target.value || undefined })}
                disabled={readOnly}
                placeholder="(inherit)"
                className={inputCls}
              />
            )}
            <div className="col-span-2 flex items-center gap-6">
              <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <input
                  type="checkbox"
                  checked={!!m.by_azure}
                  onChange={(e) => setModel({ by_azure: e.target.checked || undefined })}
                  disabled={readOnly}
                />
                Azure OpenAI
              </label>
              {m.by_azure && field('API Version', 'model-apiversion',
                <Input
                  id="model-apiversion"
                  value={m.api_version ?? ''}
                  onChange={(e) => setModel({ api_version: e.target.value || undefined })}
                  disabled={readOnly}
                  placeholder="e.g. 2024-02-01"
                  className={inputCls}
                />
              )}
            </div>
          </CardContent>
        )}
      </Card>

      {/* Brain Config */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Inference Config</CardTitle>
            {!readOnly && (
              <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <input
                  type="checkbox"
                  checked={brainEnabled}
                  onChange={(e) => onBrainChange(e.target.checked ? {} : null)}
                  className="rounded"
                />
                {toggleLabel}
              </label>
            )}
          </div>
          {!brainEnabled && (
            <p className="text-xs text-muted-foreground">{inheritMsg}</p>
          )}
        </CardHeader>
        {brainEnabled && (
          <CardContent className="grid grid-cols-3 gap-3">
            {field('Effort', 'brain-effort',
              <select
                id="brain-effort"
                value={b.effort ?? ''}
                onChange={(e) => setBrain({ effort: e.target.value || undefined })}
                disabled={readOnly}
                className={selectCls}
              >
                {EFFORT_OPTIONS.map((o) => (
                  <option key={o} value={o}>{o || '(inherit)'}</option>
                ))}
              </select>
            )}
            {field('Thinking', 'brain-thinking',
              <select
                id="brain-thinking"
                value={b.thinking ?? ''}
                onChange={(e) => setBrain({ thinking: e.target.value || undefined })}
                disabled={readOnly}
                className={selectCls}
              >
                {THINKING_OPTIONS.map((o) => (
                  <option key={o} value={o}>{o || '(inherit)'}</option>
                ))}
              </select>
            )}
            {field('Max Retries', 'brain-retries',
              <Input
                id="brain-retries"
                type="number"
                min="0"
                value={b.max_retries ?? ''}
                onChange={(e) => setBrain({ max_retries: parseInt(e.target.value) || undefined })}
                disabled={readOnly}
                placeholder="(inherit)"
                className={inputCls}
              />
            )}
          </CardContent>
        )}
      </Card>
    </div>
  )
}
