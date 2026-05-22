import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { streamRun } from '@/api/chat'
import { getSessionHistory, clearHistory } from '@/api/sessions'
import MessageList from '@/components/MessageList'
import MessageInput from '@/components/MessageInput'
import SessionSidebar from '@/components/SessionSidebar'
import { useAuth } from '@/App'

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  error?: boolean
}

export default function ChatPage() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const { role } = useAuth()
  const [messages, setMessages] = useState<Message[]>([])
  const [isStreaming, setIsStreaming] = useState(false)

  useEffect(() => {
    if (!sessionId) return
    setMessages([])
    setIsStreaming(false)
    getSessionHistory(sessionId).then((events) => {
      setMessages(
        events.map((e) => ({
          id: e.created_at + e.role,
          role: e.role,
          content: e.content,
        })),
      )
    }).catch(() => {/* session may not exist yet */})
  }, [sessionId])

  async function handleCommand(command: string) {
    if (!sessionId) return
    if (command === '/clear') {
      try {
        await clearHistory(sessionId)
      } catch {
        // best-effort: clear UI even if the API call fails
      }
      setMessages([])
    }
  }

  async function handleSend(text: string, mode: 'default' | 'plan') {
    if (!sessionId || isStreaming) return

    const userMsg: Message = { id: crypto.randomUUID(), role: 'user', content: text }
    const assistantMsg: Message = {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: '',
      streaming: true,
    }

    setMessages((prev) => [...prev, userMsg, assistantMsg])
    setIsStreaming(true)

    try {
      for await (const token of streamRun(sessionId, text, mode)) {
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantMsg.id ? { ...m, content: m.content + token } : m,
          ),
        )
      }
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMsg.id ? { ...m, streaming: false } : m,
        ),
      )
    } catch (err) {
      const errText = err instanceof Error ? err.message : 'An error occurred'
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMsg.id
            ? { ...m, content: errText, streaming: false, error: true }
            : m,
        ),
      )
    } finally {
      setIsStreaming(false)
    }
  }

  return (
    <div className="h-screen flex bg-background">
      <SessionSidebar activeSessionId={sessionId ?? ''} />

      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar */}
        <div className="border-b border-border px-6 py-3 flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-primary" />
          <span className="text-muted-foreground text-xs font-mono truncate">
            {sessionId}
          </span>
        </div>

        {role === 'viewer' && (
          <div className="px-4 py-2 bg-muted border-b text-sm text-muted-foreground text-center">
            只读模式 — 您的角色无法发送消息
          </div>
        )}

        <MessageList messages={messages} />

        <MessageInput
          onSend={handleSend}
          onCommand={handleCommand}
          disabled={isStreaming || role === 'viewer'}
          showModeToggle={role !== 'viewer'}
        />
      </div>
    </div>
  )
}
