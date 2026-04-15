'use client'

import { useState, useEffect } from 'react'
import { listFiles, FileEntry } from '@/lib/api'

interface FileBrowserProps {
  onClose: () => void
  currentPath: string
  onNavigate: (path: string) => void
}

interface BreadcrumbItem {
  name: string
  path: string
}

export default function FileBrowser({ onClose, currentPath, onNavigate }: FileBrowserProps) {
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([{ name: 'Root', path: '.' }])
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; entry: FileEntry } | null>(null)

  useEffect(() => {
    loadDirectory(currentPath)
  }, [currentPath])

  useEffect(() => {
    const handleClick = () => setContextMenu(null)
    window.addEventListener('click', handleClick)
    return () => window.removeEventListener('click', handleClick)
  }, [])

  const loadDirectory = async (path: string) => {
    setLoading(true)
    setError(null)
    try {
      const result = await listFiles(path)
      if (result.error) {
        setError(result.error)
        setEntries([])
      } else {
        // Parse the listing output
        const parsed = parseListingOutput(result.result || '')
        setEntries(parsed)
      }
    } catch (e) {
      setError('Failed to load directory')
      setEntries([])
    }
    setLoading(false)
  }

  const parseListingOutput = (output: string): FileEntry[] => {
    const lines = output.split('\n').filter(l => l.trim() && !l.startsWith('─') && !l.match(/^name\s+/))
    if (lines.length === 0) return []
    
    // Skip header line
    const entryLines = lines.filter(l => !l.startsWith('name ') && !l.includes('modified'))
    
    const entries: FileEntry[] = []
    for (const line of entryLines) {
      // Format: name                          size        modified
      const match = line.match(/^(.+?)\s+(\S+)\s*(.*)$/)
      if (match) {
        const name = match[1].trim().replace(/\/$/, '')
        const isDir = match[1].endsWith('/')
        const size = match[2]
        const modTime = match[3] || ''
        
        // Skip "dir" or "X items" as actual sizes
        if (size === 'dir' || size.includes('items')) {
          entries.push({
            name,
            size: 0,
            is_dir: true,
            mod_time: modTime,
          })
        } else {
          entries.push({
            name: name,
            size: parseSize(size),
            is_dir: isDir,
            mod_time: modTime,
          })
        }
      }
    }
    return entries
  }

  const parseSize = (sizeStr: string): number => {
    const match = sizeStr.match(/^([\d.]+)\s*(B|KB|MB|GB)?$/i)
    if (!match) return 0
    const num = parseFloat(match[1])
    const unit = match[2]?.toUpperCase() || 'B'
    switch (unit) {
      case 'KB': return num * 1024
      case 'MB': return num * 1024 * 1024
      case 'GB': return num * 1024 * 1024 * 1024
      default: return num
    }
  }

  const handleEntryClick = (entry: FileEntry) => {
    if (entry.is_dir) {
      const newPath = currentPath === '.' ? entry.name : `${currentPath}/${entry.name}`
      onNavigate(newPath)
      // Update breadcrumbs
      setBreadcrumbs(prev => [...prev, { name: entry.name, path: newPath }])
    }
  }

  const handleBreadcrumbClick = (index: number) => {
    const crumb = breadcrumbs[index]
    onNavigate(crumb.path)
    setBreadcrumbs(prev => prev.slice(0, index + 1))
  }

  const handleContextMenu = (e: React.MouseEvent, entry: FileEntry) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, entry })
  }

  const copyPath = () => {
    if (contextMenu) {
      const fullPath = currentPath === '.' ? contextMenu.entry.name : `${currentPath}/${contextMenu.entry.name}`
      navigator.clipboard.writeText(fullPath)
    }
  }

  return (
    <div className="w-72 bg-gray-900 border-r border-gray-800 flex flex-col h-full">
      {/* Header */}
      <div className="p-4 border-b border-gray-800 flex items-center justify-between">
        <h2 className="font-semibold text-sm">Files</h2>
        <button 
          onClick={onClose}
          className="p-1 hover:bg-gray-800 rounded text-gray-400 hover:text-white transition-colors"
        >
          ✕
        </button>
      </div>

      {/* Breadcrumbs */}
      <div className="px-4 py-2 border-b border-gray-800 flex items-center gap-1 overflow-x-auto text-sm">
        {breadcrumbs.map((crumb, i) => (
          <span key={i} className="flex items-center">
            {i > 0 && <span className="text-gray-600 mx-1">/</span>}
            <button
              onClick={() => handleBreadcrumbClick(i)}
              className={`hover:text-blue-400 transition-colors ${i === breadcrumbs.length - 1 ? 'text-blue-400' : 'text-gray-400'}`}
            >
              {crumb.name}
            </button>
          </span>
        ))}
      </div>

      {/* File list */}
      <div className="flex-1 overflow-y-auto p-2">
        {loading && (
          <div className="text-center text-gray-500 py-8">Loading...</div>
        )}
        
        {error && (
          <div className="text-red-400 text-sm py-4 px-2">{error}</div>
        )}

        {!loading && !error && entries.length === 0 && (
          <div className="text-center text-gray-500 py-8">(empty directory)</div>
        )}

        {!loading && !error && entries.map((entry, i) => (
          <div
            key={i}
            onClick={() => handleEntryClick(entry)}
            onContextMenu={(e) => handleContextMenu(e, entry)}
            className="flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-gray-800 cursor-pointer transition-colors group"
          >
            <span className="text-lg">{entry.is_dir ? '📁' : '📄'}</span>
            <div className="flex-1 min-w-0">
              <p className="text-sm truncate">{entry.name}</p>
              {!entry.is_dir && (
                <p className="text-xs text-gray-500">{formatFileSize(entry.size)}</p>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Context menu */}
      {contextMenu && (
        <div
          className="fixed bg-gray-800 border border-gray-700 rounded-lg shadow-xl py-1 z-50 min-w-48"
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          <button
            onClick={copyPath}
            className="w-full text-left px-4 py-2 text-sm hover:bg-gray-700 text-gray-300"
          >
            📋 Copy path
          </button>
          {contextMenu.entry.is_dir && (
            <button
              onClick={() => {
                const newPath = currentPath === '.' ? contextMenu.entry.name : `${currentPath}/${contextMenu.entry.name}`
                onNavigate(newPath)
                setBreadcrumbs(prev => [...prev, { name: contextMenu.entry.name, path: newPath }])
                setContextMenu(null)
              }}
              className="w-full text-left px-4 py-2 text-sm hover:bg-gray-700 text-gray-300"
            >
              📂 Open
            </button>
          )}
        </div>
      )}

      {/* Footer */}
      <div className="p-4 border-t border-gray-800 text-xs text-gray-500">
        <p>Right-click for options</p>
      </div>
    </div>
  )
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}