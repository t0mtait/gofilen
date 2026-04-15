'use client'

import { useState, useEffect } from 'react'
import { getConfig, getPing } from '@/lib/api'

interface SettingsPanelProps {
  onClose: () => void
}

export default function SettingsPanel({ onClose }: SettingsPanelProps) {
  const [config, setConfig] = useState<Record<string, string>>({})
  const [ping, setPing] = useState<{ status: string; webdav_online: boolean; action_history?: string } | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadSettings()
  }, [])

  const loadSettings = async () => {
    setLoading(true)
    try {
      const [cfg, p] = await Promise.all([getConfig(), getPing()])
      setConfig(cfg)
      setPing(p)
    } catch (e) {
      console.error('Failed to load settings', e)
    }
    setLoading(false)
  }

  const handleSave = () => {
    // In a real app, this would save to the server
    alert('Settings saved! (Demo - actual save not implemented)')
    onClose()
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-gray-900 rounded-xl border border-gray-700 w-full max-w-lg mx-4 max-h-[80vh] overflow-y-auto">
        {/* Header */}
        <div className="p-4 border-b border-gray-800 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Settings</h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-800 rounded-lg transition-colors text-gray-400 hover:text-white"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-6">
          {loading ? (
            <div className="text-center text-gray-500 py-8">Loading...</div>
          ) : (
            <>
              {/* Status */}
              <div>
                <h3 className="text-sm font-medium text-gray-400 mb-3">Connection Status</h3>
                <div className="space-y-2">
                  <div className="flex items-center justify-between p-3 bg-gray-800 rounded-lg">
                    <span className="text-sm">WebDAV Server</span>
                    <span className={`text-sm font-medium ${ping?.webdav_online ? 'text-green-400' : 'text-red-400'}`}>
                      {ping?.webdav_online ? '● Online' : '○ Offline'}
                    </span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-gray-800 rounded-lg">
                    <span className="text-sm">Server Status</span>
                    <span className="text-sm font-medium text-green-400">
                      {ping?.status === 'ok' ? '● Running' : '○ Error'}
                    </span>
                  </div>
                </div>
              </div>

              {/* Model Settings */}
              <div>
                <h3 className="text-sm font-medium text-gray-400 mb-3">AI Model</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Model Name</label>
                    <input
                      type="text"
                      value={config.model || ''}
                      onChange={e => setConfig({ ...config, model: e.target.value })}
                      className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                      placeholder="llama3.2"
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Ollama URL</label>
                    <input
                      type="text"
                      value={config.ollama_url || ''}
                      onChange={e => setConfig({ ...config, ollama_url: e.target.value })}
                      className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                      placeholder="http://localhost:11434"
                    />
                  </div>
                </div>
              </div>

              {/* WebDAV Settings */}
              <div>
                <h3 className="text-sm font-medium text-gray-400 mb-3">WebDAV</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">WebDAV URL</label>
                    <input
                      type="text"
                      value={config.webdav_url || ''}
                      onChange={e => setConfig({ ...config, webdav_url: e.target.value })}
                      className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                      placeholder="http://localhost:8080"
                    />
                  </div>
                </div>
              </div>

              {/* Server Port */}
              <div>
                <h3 className="text-sm font-medium text-gray-400 mb-3">Server</h3>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Port</label>
                  <input
                    type="text"
                    value={config.server_port || ''}
                    onChange={e => setConfig({ ...config, server_port: e.target.value })}
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                    placeholder="3001"
                  />
                </div>
              </div>

              {/* Action History */}
              {ping?.action_history && (
                <div>
                  <h3 className="text-sm font-medium text-gray-400 mb-3">Action History</h3>
                  <pre className="bg-gray-800 rounded-lg p-3 text-xs text-gray-400 overflow-x-auto max-h-40">
                    {ping.action_history}
                  </pre>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-800 flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-400 hover:text-white transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>
  )
}