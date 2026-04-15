'use client'

import { useState, useRef, useEffect, useCallback } from 'react'
import { streamChat, ChatMessage, StreamEvent } from '@/lib/api'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'error' | 'system'
  content: string
  toolCall?: string
  toolArgs?: string
  toolResult?: string
}

interface ChatProps {
  currentPath: string
  onPathChange: (path: string) => void
}

export default function Chat({ currentPath, onPathChange }: ChatProps) {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [currentEvent, setCurrentEvent] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault()
    const content = input.trim()
    if (!content || streaming) return

    const userMessage: Message = {
      id: Date.now().toString() + '-user',
      role: 'user',
      content,
    }
    setMessages(prev => [...prev, userMessage])
    setInput('')
    setStreaming(true)

    const history: ChatMessage[] = messages
      .filter(m => m.role === 'user' || m.role === 'assistant')
      .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))

    try {
      const assistantMessage: Message = {
        id: Date.now().toString() + '-assistant',
        role: 'assistant',
        content: '',
      }
      setMessages(prev => [...prev, assistantMessage])

      let lastContent = ''
      for await (const event of streamChat(content, history)) {
        setCurrentEvent(event.type)

        switch (event.type) {
          case 'chunk':
            lastContent += event.content || ''
            setMessages(prev => prev.map(m => 
              m.id === assistantMessage.id 
                ? { ...m, content: lastContent }
                : m
            ))
            break
          case 'tool_call':
            setMessages(prev => [...prev, {
              id: Date.now().toString() + '-tool',
              role: 'system',
              content: '',
              toolCall: event.name,
              toolArgs: event.args,
            }])
            break
          case 'tool_result':
            setMessages(prev => prev.map(m => 
              m.toolCall === event.name && m.toolArgs === event.args && !m.toolResult
                ? { ...m, toolResult: event.result }
                : m
            ))
            break
          case 'done':
          case 'error':
            setStreaming(false)
            setCurrentEvent('')
            break
        }
      }
    } catch (err) {
      setMessages(prev => [...prev, {
        id: Date.now().toString() + '-error',
        role: 'error',
        content: err instanceof Error ? err.message : 'An error occurred',
      }])
    }

    setStreaming(false)
    setCurrentEvent('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit()
    }
  }

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Messages area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="text-center text-gray-500 mt-20">
            <p className="text-lg mb-2">👋 Welcome to gofilen</p>
            <p className="text-sm">
              Ask me to list files, read documents, create folders, or manage your Filen cloud drive.
            </p>
            <p className="text-sm mt-2 text-gray-600">
              Current path: <code className="bg-gray-800 px-2 py-1 rounded">{currentPath}</code>
            </p>
          </div>
        )}

        {messages.map((msg) => (
          <div key={msg.id}>
            {msg.role === 'user' ? (
              <div className="flex justify-end">
                <div className="bg-blue-600 text-white rounded-2xl rounded-br-md px-4 py-2 max-w-xl">
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
              </div>
            ) : msg.role === 'assistant' ? (
              <div className="flex justify-start">
                <div className="bg-gray-800 rounded-2xl rounded-bl-md px-4 py-2 max-w-2xl">
                  <p className="whitespace-pre-wrap font-mono text-sm">{msg.content}</p>
                </div>
              </div>
            ) : msg.role === 'error' ? (
              <div className="flex justify-start">
                <div className="bg-red-900 text-red-200 rounded-2xl rounded-bl-md px-4 py-2 max-w-xl">
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
              </div>
            ) : msg.toolCall ? (
              <div className="ml-4 border-l-2 border-blue-500 pl-4 py-2">
                <div className="text-blue-400 text-sm font-mono">
                  🔧 {msg.toolCall}
                </div>
                {msg.toolArgs && (
                  <div className="text-gray-500 text-xs mt-1 font-mono">
                    Args: {msg.toolArgs}
                  </div>
                )}
                {msg.toolResult && (
                  <div className="mt-2 text-green-400 text-xs font-mono bg-gray-900 rounded p-2 max-h-48 overflow-y-auto">
                    <pre className="whitespace-pre-wrap">{msg.toolResult}</pre>
                  </div>
                )}
              </div>
            ) : null}
          </div>
        ))}

        {streaming && currentEvent === 'chunk' && (
          <div className="flex justify-start">
            <div className="bg-gray-800 rounded-2xl rounded-bl-md px-4 py-2">
              <span className="animate-pulse">...</span>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="border-t border-gray-800 p-4 bg-gray-900">
        <form onSubmit={handleSubmit} className="flex gap-3">
          <textarea
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`Ask about files in ${currentPath === '.' ? 'root' : currentPath}...`}
            className="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-4 py-3 resize-none text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
            rows={1}
            disabled={streaming}
            style={{ minHeight: '48px', maxHeight: '120px' }}
          />
          <button
            type="submit"
            disabled={!input.trim() || streaming}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:text-gray-500 text-white px-6 py-3 rounded-xl font-medium transition-colors"
          >
            {streaming ? '...' : 'Send'}
          </button>
        </form>
        <p className="text-xs text-gray-600 mt-2 text-center">
          Press Enter to send, Shift+Enter for new line
        </p>
      </div>
    </div>
  )
}