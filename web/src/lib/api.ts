const API_BASE = ''

export interface FileEntry {
  name: string
  size: number
  is_dir: boolean
  mod_time: string
}

export interface ChatMessage {
  role: 'user' | 'assistant' | 'error'
  content: string
}

export interface ToolCall {
  tool_call?: string
  tool_args?: string
  tool_result?: string
}

export interface StreamEvent {
  type: 'chunk' | 'tool_call' | 'tool_result' | 'done' | 'error'
  content?: string
  name?: string
  args?: string
  result?: string
  message?: string
}

export async function listFiles(path: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/list`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
  return res.json()
}

export async function readFile(path: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/read`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
  return res.json()
}

export async function writeFile(path: string, content: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/write`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, content }),
  })
  return res.json()
}

export async function deleteFile(path: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
  return res.json()
}

export async function mkdir(path: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/mkdir`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
  return res.json()
}

export async function moveFile(src: string, dst: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/move`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ src, dst }),
  })
  return res.json()
}

export async function copyFile(src: string, dst: string): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/copy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ src, dst }),
  })
  return res.json()
}

export async function tree(depth: number = 3): Promise<{ result?: string; error?: string }> {
  const res = await fetch(`${API_BASE}/api/files/tree`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ depth }),
  })
  return res.json()
}

export async function getConfig(): Promise<Record<string, string>> {
  const res = await fetch(`${API_BASE}/api/config`)
  return res.json()
}

export async function getPing(): Promise<{ status: string; webdav_online: boolean; action_history?: string }> {
  const res = await fetch(`${API_BASE}/api/ping`)
  return res.json()
}

export async function* streamChat(message: string, history: ChatMessage[]): AsyncGenerator<StreamEvent> {
  const response = await fetch(`${API_BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message, history }),
  })

  if (!response.body) throw new Error('No response body')

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let currentEvent = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      if (line.startsWith('event: ')) {
        currentEvent = line.slice(7).trim()
      } else if (line.startsWith('data: ')) {
        try {
          const data = JSON.parse(line.slice(6))
          yield { type: currentEvent as StreamEvent['type'], ...data }
        } catch {}
      }
    }
  }
}