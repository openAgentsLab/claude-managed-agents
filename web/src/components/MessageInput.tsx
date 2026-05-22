import { useState, type KeyboardEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

type Mode = 'default' | 'plan'

interface SlashCommand {
  command: string
  description: string
}

const SLASH_COMMANDS: SlashCommand[] = [
  { command: '/clear', description: '清除对话历史记录' },
]

interface Props {
  onSend: (text: string, mode: Mode) => void
  onCommand?: (command: string) => void
  disabled?: boolean
  showModeToggle?: boolean
}

export default function MessageInput({ onSend, onCommand, disabled, showModeToggle = true }: Props) {
  const [text, setText] = useState('')
  const [mode, setMode] = useState<Mode>('default')
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [dismissed, setDismissed] = useState(false)

  const trimmed = text.trim()
  const filtered =
    trimmed.startsWith('/') && !trimmed.includes(' ')
      ? SLASH_COMMANDS.filter((c) => c.command.startsWith(trimmed))
      : []
  const showPopup = filtered.length > 0 && !dismissed

  function handleTextChange(value: string) {
    setText(value)
    setSelectedIdx(0)
    setDismissed(false)
  }

  function submit(msg?: string) {
    const toSend = (msg ?? text).trim()
    if (!toSend || disabled) return
    if (toSend.startsWith('/') && !toSend.includes(' ') && onCommand) {
      onCommand(toSend)
    } else {
      onSend(toSend, mode)
    }
    setText('')
    setDismissed(false)
    setSelectedIdx(0)
  }

  function pickCommand(cmd: string) {
    setText(cmd)
    setDismissed(false)
    setSelectedIdx(0)
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (showPopup) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIdx((i) => Math.min(i + 1, filtered.length - 1))
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIdx((i) => Math.max(i - 1, 0))
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setDismissed(true)
        return
      }
      if (e.key === 'Tab') {
        e.preventDefault()
        pickCommand(filtered[selectedIdx]?.command ?? trimmed)
        return
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        const cmd = filtered[selectedIdx]?.command
        if (cmd) submit(cmd)
        return
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  return (
    <div className="border-t border-border px-6 py-4">
      <div className="max-w-3xl mx-auto space-y-2">
        {showModeToggle && (
          <div className="flex items-center gap-2">
            <label htmlFor="mode-select" className="text-xs text-muted-foreground">Mode</label>
            <select
              id="mode-select"
              value={mode}
              onChange={(e) => setMode(e.target.value as Mode)}
              className="h-7 rounded-md border border-input bg-background px-2 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              <option value="default">Default</option>
              <option value="plan">Plan</option>
            </select>
          </div>
        )}

        <div className="relative flex items-end gap-3">
          {showPopup && (
            <div className="absolute bottom-full left-0 mb-2 w-[calc(100%-52px)] rounded-lg border border-border bg-popover shadow-md overflow-hidden z-50">
              {filtered.map((cmd, i) => (
                <button
                  type="button"
                  key={cmd.command}
                  onClick={() => submit(cmd.command)}
                  className={`w-full flex items-center gap-3 px-3 py-2 text-sm text-left transition-colors ${
                    i === selectedIdx
                      ? 'bg-accent text-accent-foreground'
                      : 'hover:bg-accent/50'
                  }`}
                >
                  <span className="font-mono font-medium text-primary">{cmd.command}</span>
                  <span className="text-muted-foreground">{cmd.description}</span>
                </button>
              ))}
            </div>
          )}

          <Textarea
            value={text}
            onChange={(e) => handleTextChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder={disabled ? 'Waiting for response…' : 'Message Forge… (/ for commands)'}
            rows={1}
            className="flex-1 min-h-[44px] max-h-48 resize-none text-sm"
            onInput={(e) => {
              const el = e.currentTarget
              el.style.height = 'auto'
              el.style.height = `${Math.min(el.scrollHeight, 192)}px`
            }}
          />
          <Button
            onClick={() => submit()}
            disabled={disabled || !text.trim()}
            size="icon"
            className="h-11 w-11 flex-shrink-0"
          >
            <svg viewBox="0 0 16 16" fill="none" className="w-4 h-4">
              <path
                d="M8 2.5L13.5 8 8 13.5M13.5 8H2.5"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </Button>
        </div>
      </div>
      <p className="text-xs text-muted-foreground text-center mt-2 max-w-3xl mx-auto">
        Forge can make mistakes. Review important information.
      </p>
    </div>
  )
}
