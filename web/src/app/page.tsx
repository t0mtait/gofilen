'use client'

import { useState, useRef, useEffect } from 'react'
import Chat from '@/components/Chat'
import FileBrowser from '@/components/FileBrowser'
import SettingsPanel from '@/components/SettingsPanel'

export default function Home() {
  const [showFiles, setShowFiles] = useState(true)
  const [showSettings, setShowSettings] = useState(false)
  const [currentPath, setCurrentPath] = useState('.')

  return (
    <div className="flex h-screen bg-gray-950 text-gray-100">
      {showFiles && (
        <FileBrowser 
          onClose={() => setShowFiles(false)} 
          currentPath={currentPath}
          onNavigate={setCurrentPath}
        />
      )}
      <main className="flex-1 flex flex-col min-w-0">
        <header className="flex items-center gap-3 p-4 border-b border-gray-800 bg-gray-900">
          <button 
            onClick={() => setShowFiles(!showFiles)}
            className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
            title="Toggle file browser"
          >
            📁
          </button>
          <h1 className="text-lg font-semibold">gofilen</h1>
          <div className="ml-auto flex items-center gap-2">
            <span className="text-sm text-gray-500">{currentPath === '.' ? 'Root' : currentPath}</span>
            <button 
              onClick={() => setShowSettings(true)}
              className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
              title="Settings"
            >
              ⚙️
            </button>
          </div>
        </header>
        <Chat currentPath={currentPath} onPathChange={setCurrentPath} />
      </main>
      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} />}
    </div>
  )
}