import { useEffect, useRef } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@/lib/utils'

export interface MessageSegment {
  type: 'token' | 'tool_use' | 'status' | 'task_update'
  content: string
  tool?: string
  description?: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  segments?: MessageSegment[]
  streaming?: boolean
  error?: boolean
}

interface Props {
  messages: Message[]
}

const Cursor = () => (
  <span className="inline-block w-1.5 h-4 bg-muted-foreground rounded-sm ml-0.5 animate-pulse align-middle" />
)

const mdComponents: Components = {
  p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
  h1: ({ children }) => <h1 className="text-xl font-bold mt-3 mb-2 first:mt-0">{children}</h1>,
  h2: ({ children }) => <h2 className="text-lg font-bold mt-3 mb-2 first:mt-0">{children}</h2>,
  h3: ({ children }) => <h3 className="text-base font-semibold mt-2 mb-1 first:mt-0">{children}</h3>,
  h4: ({ children }) => <h4 className="text-sm font-semibold mt-2 mb-1 first:mt-0">{children}</h4>,
  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  ul: ({ children }) => <ul className="list-disc pl-5 mb-2 space-y-0.5">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal pl-5 mb-2 space-y-0.5">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-muted-foreground/40 pl-3 italic text-muted-foreground mb-2">
      {children}
    </blockquote>
  ),
  a: ({ href, children }) => (
    <a href={href} className="text-primary underline underline-offset-2 hover:opacity-80" target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  ),
  hr: () => <hr className="border-border my-3" />,
  code: ({ className, children, ...props }) => {
    const isBlock = Boolean(className)
    if (isBlock) {
      return <code className={cn('text-xs font-mono', className)} {...props}>{children}</code>
    }
    return (
      <code className="bg-muted px-1 py-0.5 rounded text-[0.8em] font-mono" {...props}>
        {children}
      </code>
    )
  },
  pre: ({ children }) => (
    <pre className="bg-muted rounded-lg p-3 overflow-x-auto text-xs font-mono mb-2 whitespace-pre">
      {children}
    </pre>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto mb-2">
      <table className="border-collapse text-xs w-full">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border border-border px-2 py-1 bg-muted font-semibold text-left">{children}</th>
  ),
  td: ({ children }) => (
    <td className="border border-border px-2 py-1">{children}</td>
  ),
}

function SegmentBubble({ seg, showCursor }: { seg: MessageSegment; showCursor?: boolean }) {
  return (
    <div className="rounded-2xl px-4 py-3 text-sm bg-card border border-border text-card-foreground rounded-tl-sm">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
        {seg.content}
      </ReactMarkdown>
      {showCursor && <Cursor />}
    </div>
  )
}

function ActivityRow({ seg, showCursor }: { seg: MessageSegment; showCursor?: boolean }) {
  const label = seg.content
  return (
    <div className="flex items-center gap-2 px-1 py-0.5 text-xs text-muted-foreground/60 min-w-0">
      <span className="w-1 h-1 rounded-full bg-muted-foreground/30 flex-shrink-0" />
      {seg.type === 'tool_use' ? (
        <>
          <code className="font-mono text-[11px] text-foreground/50 shrink-0">{seg.tool ?? label}</code>
          {seg.description && <span className="truncate">{seg.description}</span>}
        </>
      ) : (
        <span>{label}</span>
      )}
      {showCursor && <Cursor />}
    </div>
  )
}

function AssistantSegments({ segments, streaming }: { segments: MessageSegment[]; streaming?: boolean }) {
  const lastIdx = segments.length - 1
  return (
    <div className="flex flex-col gap-1.5">
      {segments.map((seg, i) =>
        seg.type === 'token' ? (
          <SegmentBubble key={i} seg={seg} showCursor={streaming && i === lastIdx} />
        ) : (
          <ActivityRow key={i} seg={seg} showCursor={streaming && i === lastIdx} />
        ),
      )}
      {streaming && segments.length === 0 && (
        <div className="rounded-2xl px-4 py-3 text-sm bg-card border border-border text-card-foreground rounded-tl-sm">
          <Cursor />
        </div>
      )}
    </div>
  )
}

export default function MessageList({ messages }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-center">
          <div className="w-10 h-10 rounded-xl bg-muted flex items-center justify-center mx-auto mb-3">
            <svg viewBox="0 0 24 24" fill="none" className="w-5 h-5 text-muted-foreground" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
            </svg>
          </div>
          <p className="text-muted-foreground text-sm">How can I help you today?</p>
        </div>
      </div>
    )
  }

  return (
    <ScrollArea className="flex-1 min-h-0">
      <div className="px-6 py-6 space-y-6 max-w-3xl mx-auto">
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : 'flex-row'}`}
          >
            <div
              className={`w-7 h-7 rounded-full flex-shrink-0 flex items-center justify-center text-xs font-semibold mt-0.5 ${
                msg.role === 'user'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground border border-border'
              }`}
            >
              {msg.role === 'user' ? 'U' : 'F'}
            </div>

            <div className={`flex-1 max-w-[85%] ${msg.role === 'user' ? 'items-end' : 'items-start'} flex flex-col gap-1`}>
              {msg.role === 'user' ? (
                <div className="rounded-2xl px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap break-words bg-primary text-primary-foreground rounded-tr-sm">
                  {msg.content}
                </div>
              ) : msg.error ? (
                <div className="rounded-2xl px-4 py-3 text-sm leading-relaxed bg-destructive/10 border border-destructive/30 text-destructive rounded-tl-sm">
                  {msg.content}
                </div>
              ) : msg.segments !== undefined ? (
                <AssistantSegments segments={msg.segments} streaming={msg.streaming} />
              ) : (
                <div className="rounded-2xl px-4 py-3 text-sm bg-card border border-border text-card-foreground rounded-tl-sm">
                  <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
                    {msg.content}
                  </ReactMarkdown>
                </div>
              )}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </ScrollArea>
  )
}
