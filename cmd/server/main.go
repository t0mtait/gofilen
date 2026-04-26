// Package server provides the HTTP server for gofilen.
// It is imported by the root main.go.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/t0mtait/gofilen/internal/config"
	"github.com/t0mtait/gofilen/internal/filer"
	"github.com/t0mtait/gofilen/internal/llm"
)

var version = "dev"

// RunServer runs the HTTP server with the given config.
// It blocks until the server exits.
func RunServer(cfg config.Config) error {
	if errs := cfg.Validate(); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "config error: %s\n", err)
		}
		return fmt.Errorf("invalid config")
	}

	if cfg.HasWebDAVCredentials() {
		log.Printf("Starting Filen WebDAV server...")
		if err := cfg.StartWebDAVServer(); err != nil {
			log.Printf("Warning: could not start WebDAV server: %v", err)
		} else {
			log.Printf("WebDAV server ready at %s", cfg.WebDAVURL)
		}
	}

	var f filer.Filer
	var err error
	if cfg.HasWebDAVCredentials() {
		f, err = filer.NewWebDAV(cfg.WebDAVURL, cfg.WebDAVUser, cfg.WebDAVPassword)
		if err != nil {
			log.Fatalf("Failed to create WebDAV Filer: %v", err)
		}
		log.Printf("Connected to WebDAV at %s", cfg.WebDAVURL)
	} else {
		f, err = filer.NewLocal(cfg.Dir)
		if err != nil {
			log.Fatalf("Failed to create local Filer: %v", err)
		}
		log.Printf("Using local directory: %s", cfg.Dir)
	}

	llmClient := llm.NewClient(cfg.OllamaURL, cfg.Model)
	tools := llm.FileTools()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/ping", withError(pingHandler(f, llmClient)))
	mux.HandleFunc("GET /api/config", withError(configHandler(cfg)))
	mux.HandleFunc("POST /api/chat", withError(chatHandler(f, llmClient, tools)))
	mux.HandleFunc("POST /api/files/list", withError(filesListHandler(f)))
	mux.HandleFunc("POST /api/files/read", withError(filesReadHandler(f)))
	mux.HandleFunc("POST /api/files/write", withError(filesWriteHandler(f)))
	mux.HandleFunc("POST /api/files/delete", withError(filesDeleteHandler(f)))
	mux.HandleFunc("POST /api/files/mkdir", withError(filesMkdirHandler(f)))
	mux.HandleFunc("POST /api/files/move", withError(filesMoveHandler(f)))
	mux.HandleFunc("POST /api/files/copy", withError(filesCopyHandler(f)))
	mux.HandleFunc("POST /api/files/tree", withError(filesTreeHandler(f)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			serveWebUI(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
	})

	handler := corsMiddleware(mux)

	log.Printf("Server starting on http://localhost:%s", cfg.ServerPort)
	addr := ":" + cfg.ServerPort
	if err := http.ListenAndServe(addr, handler); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func withError(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			log.Printf("Handler error: %v", err)
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	writeJSON(w, map[string]string{"error": message})
}

// --- API Handlers ---

func pingHandler(f filer.Filer, _ *llm.Client) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		resp := map[string]any{
			"status":        "ok",
			"webdav_online": false,
		}
		if f != nil {
			if err := f.Ping(); err == nil {
				resp["webdav_online"] = true
			}
		}
		resp["action_history"] = f.ActionHistory()
		writeJSON(w, resp)
		return nil
	}
}

func configHandler(cfg config.Config) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		redacted := cfg.Redact()
		writeJSON(w, map[string]string{
			"model":       redacted.Model,
			"ollama_url":  redacted.OllamaURL,
			"webdav_url":  redacted.WebDAVURL,
			"server_port": redacted.ServerPort,
		})
		return nil
	}
}

type chatRequest struct {
	Message string              `json:"message"`
	History []map[string]string `json:"history"`
}

func chatHandler(f filer.Filer, llmClient *llm.Client, tools []llm.Tool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.Header.Get("Content-Type") != "application/json" {
			return fmt.Errorf("expected Content-Type: application/json")
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		messages := make([]llm.Message, 0, len(req.History))
		for _, h := range req.History {
			messages = append(messages, llm.Message{
				Role:    llm.Role(h["role"]),
				Content: h["content"],
			})
		}
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: req.Message,
		})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return fmt.Errorf("streaming not supported")
		}

		ctx := r.Context()
		ch := make(chan llm.StreamEvent, 64)

		go llm.RunConversation(ctx, llmClient, messages, tools, f, ch)

		for e := range ch {
			switch e.Type {
			case "chunk":
				sendSSE(w, flusher, "chunk", map[string]string{"content": e.Content})
			case "tool_call":
				sendSSE(w, flusher, "tool_call", map[string]string{
					"name": e.ToolName,
					"args": e.ToolArgs,
				})
			case "tool_result":
				sendSSE(w, flusher, "tool_result", map[string]string{
					"name":   e.ToolName,
					"result": e.ToolResult,
				})
			case "done":
				sendSSE(w, flusher, "done", map[string]string{})
				return nil
			case "error":
				sendSSE(w, flusher, "error", map[string]string{"message": e.Err.Error()})
				return nil
			}
		}
		return nil
	}
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data map[string]string) {
	fmt.Fprintf(w, "event: %s\n", event)
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

// --- File API Handlers ---

type pathRequest struct {
	Path string `json:"path"`
}

func filesListHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req pathRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Path = "."
		}
		if req.Path == "" {
			req.Path = "."
		}

		// Get raw listing for backward compatibility
		result, err := f.List(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}

		// Get structured file list
		files, err := f.ListFiles(req.Path)
		if err != nil {
			writeJSON(w, map[string]interface{}{"success": true, "listing": result, "files": []interface{}{}, "error": err.Error()})
			return nil
		}

		writeJSON(w, map[string]interface{}{
			"success": true,
			"listing": result,
			"files":   files,
		})
		return nil
	}
}

func filesReadHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req pathRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.ReadFile(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

type writeRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func filesWriteHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req writeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.WriteFile(req.Path, req.Content)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

func filesDeleteHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req pathRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.Delete(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

func filesMkdirHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req pathRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.CreateDir(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

type moveRequest struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func filesMoveHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.Move(req.Src, req.Dst)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

type copyRequest struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func filesCopyHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req copyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fmt.Errorf("invalid request: %w", err)
		}
		result, err := f.Copy(req.Src, req.Dst)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

type treeRequest struct {
	Depth int `json:"depth"`
}

func filesTreeHandler(f filer.Filer) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req treeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Invalid request — use default depth
			req.Depth = 3
		} else if req.Depth < 1 || req.Depth > 10 {
			req.Depth = 3
		}
		result := f.Tree(req.Depth)
		writeJSON(w, map[string]string{"result": result})
		return nil
	}
}

// --- Web UI Serving ---

func serveWebUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>gofilen</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; height: 100vh; overflow: hidden; display: flex; }
    #app { display: flex; flex-direction: column; width: 100%; height: 100vh; }

    /* Sidebar */
    #sidebar { width: 260px; background: #0c1322; border-right: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; overflow: hidden; }
    #sidebar-header { padding: 16px; border-bottom: 1px solid rgba(255,255,255,0.06); display: flex; align-items: center; gap: 8px; }
    #sidebar-header h1 { font-size: 1rem; font-weight: 700; color: #e2e8f0; flex: 1; }
    .sidebar-btn { background: none; border: none; color: #64748b; cursor: pointer; padding: 6px; border-radius: 6px; font-size: 0.75rem; }
    .sidebar-btn:hover { background: rgba(255,255,255,0.06); color: #e2e8f0; }
    #file-list { flex: 1; overflow-y: auto; padding: 8px; }
    .file-item { padding: 6px 10px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; color: #94a3b8; display: flex; align-items: center; gap: 6px; }
    .file-item:hover { background: rgba(255,255,255,0.05); color: #e2e8f0; }
    .file-item.folder { color: #60a5fa; }
    .file-item.folder::before { content: '📁'; }
    .file-item.file::before { content: '📄'; }
    #status-bar { padding: 10px 16px; border-top: 1px solid rgba(255,255,255,0.06); font-size: 0.7rem; color: #475569; }
    .status-ok { color: #22c55e; }
    .status-err { color: #ef4444; }

    /* Chat area */
    #chat-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
    #messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 4px; }

    .msg { padding: 10px 14px; border-radius: 10px; font-size: 0.875rem; line-height: 1.5; max-width: 80%; white-space: pre-wrap; word-break: break-word; }
    .msg.user { background: #1e40af; color: #f8fafc; align-self: flex-end; border-bottom-right-radius: 2px; }
    .msg.assistant { background: #1e293b; color: #e2e8f0; align-self: flex-start; border-bottom-left-radius: 2px; }
    .msg.error { background: rgba(239,68,68,0.15); color: #fca5a5; border: 1px solid rgba(239,68,68,0.3); }
    .msg-tool { background: #1e293b; border-radius: 8px; padding: 10px 14px; margin: 4px 0; border-left: 3px solid #3b82f6; font-size: 0.8rem; }
    .msg-tool-name { color: #60a5fa; font-weight: 600; margin-bottom: 2px; }
    .msg-tool-args { color: #94a3b8; font-size: 0.75rem; }
    .msg-result { background: rgba(34,197,94,0.08); border-radius: 8px; padding: 10px 14px; margin: 4px 0; border-left: 3px solid #22c55e; font-size: 0.8rem; color: #86efac; max-height: 200px; overflow-y: auto; }

    #welcome { text-align: center; padding: 40px 20px; color: #475569; }
    #welcome h2 { font-size: 1.25rem; color: #64748b; margin-bottom: 8px; }
    #welcome p { font-size: 0.875rem; line-height: 1.6; }

    #input-area { padding: 12px 16px; border-top: 1px solid rgba(255,255,255,0.06); display: flex; gap: 8px; align-items: flex-end; }
    #messageInput { flex: 1; background: #1e293b; border: 1px solid rgba(255,255,255,0.1); border-radius: 10px; padding: 10px 14px; color: #f8fafc; font-size: 0.875rem; resize: none; outline: none; min-height: 42px; max-height: 120px; font-family: inherit; }
    #messageInput:focus { border-color: #3b82f6; }
    #messageInput::placeholder { color: #475569; }
    #sendBtn { background: #3b82f6; border: none; border-radius: 10px; padding: 10px 16px; color: white; cursor: pointer; font-size: 0.875rem; font-weight: 600; transition: opacity 0.2s; }
    #sendBtn:hover { opacity: 0.85; }
    #sendBtn:disabled { opacity: 0.4; cursor: not-allowed; }

    /* Connection banner */
    #conn-banner { background: rgba(239,68,68,0.15); border-bottom: 1px solid rgba(239,68,68,0.3); padding: 8px 16px; font-size: 0.8rem; color: #fca5a5; display: none; }
    #conn-banner.show { display: block; }
  </style>
</head>
<body>
<div id="app">
  <div id="sidebar">
    <div id="sidebar-header">
      <h1>📂 gofilen</h1>
    </div>
    <div id="file-list">
      <div id="welcome-files" style="padding:12px 8px;color:#475569;font-size:0.8rem;text-align:center;">
        Connecting to Filen...
      </div>
    </div>
    <div id="status-bar">
      <span id="conn-status">Checking connection...</span>
    </div>
  </div>
  <div id="chat-area">
    <div id="conn-banner">⚠️ Cannot connect to gofilen server. Is it running?</div>
    <div id="messages">
      <div id="welcome">
        <h2>gofilen</h2>
        <p>Your AI-powered Filen file manager.<br>Ask me to list, read, write, or organize your files.</p>
      </div>
    </div>
    <div id="input-area">
      <textarea id="messageInput" placeholder="Ask me anything about your files..." rows="1"></textarea>
      <button id="sendBtn">Send</button>
    </div>
  </div>
</div>
<script>
(function() {
  let messages = [];
  let streaming = false;
  let connected = false;

  function api(path, opts) {
    opts = opts || {};
    var ctrl = new AbortController();
    var timeout = setTimeout(function() { ctrl.abort(); }, 15000);
    opts.signal = ctrl.signal;
    return fetch(path, opts).finally(function() { clearTimeout(timeout); });
  }

  function escapeHtml(s) {
    if (!s) return '';
    return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>');
  }

  function renderMessages() {
    var container = document.getElementById('messages');
    var welcome = document.getElementById('welcome');
    if (!container) return;
    if (messages.length === 0) {
      welcome.style.display = '';
      container.innerHTML = '';
      container.appendChild(welcome);
      return;
    }
    welcome.style.display = 'none';
    container.innerHTML = '';
    for (var i = 0; i < messages.length; i++) {
      var m = messages[i];
      var div = document.createElement('div');
      if (m.tool_call) {
        div.className = 'msg-tool';
        div.innerHTML = '<div class="msg-tool-name">🔧 ' + escapeHtml(m.tool_call) + '</div>' +
          '<div class="msg-tool-args">Args: ' + escapeHtml(m.tool_args) + '</div>';
      } else if (m.tool_result) {
        div.className = 'msg-result';
        div.textContent = m.tool_result;
      } else if (m.error) {
        div.className = 'msg error';
        div.textContent = 'Error: ' + m.error;
      } else {
        div.className = 'msg ' + (m.role === 'user' ? 'user' : 'assistant');
        div.textContent = m.content || '';
      }
      container.appendChild(div);
    }
    container.scrollTop = container.scrollHeight;
  }

  function appendMsg(m) {
    messages.push(m);
    renderMessages();
  }

  async function sendMessage() {
    var input = document.getElementById('messageInput');
    var sendBtn = document.getElementById('sendBtn');
    var content = input.value.trim();
    if (!content || streaming) return;
    input.value = '';
    input.style.height = 'auto';
    streaming = true;
    sendBtn.disabled = true;

    appendMsg({ role: 'user', content: content });
    appendMsg({ role: 'assistant', content: '' });

    try {
      var resp = await api('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: content, history: [] })
      });

      if (!resp.ok) {
        var errData = await resp.json().catch(function() { return {}; });
        messages[messages.length - 1].error = (errData.detail || 'Server error: ' + resp.status);
        renderMessages();
        return;
      }

      var reader = resp.body.getReader();
      var decoder = new TextDecoder();
      var lastRole = 'chunk';
      var lastContent = '';

      while (true) {
        var result = await reader.read();
        if (result.done) break;
        var text = decoder.decode(result.value);
        var lines = text.split('\n');
        for (var j = 0; j < lines.length; j++) {
          var line = lines[j];
          if (line.indexOf('event:') === 0) {
            lastRole = line.slice(7).trim();
          } else if (line.indexOf('data:') === 0) {
            try {
              var data = JSON.parse(line.slice(6));
              if (lastRole === 'chunk' && data.content !== undefined) {
                lastContent += data.content;
                messages[messages.length - 1].content = lastContent;
                renderMessages();
              } else if (lastRole === 'tool_call') {
                appendMsg({ tool_call: data.name, tool_args: data.args });
              } else if (lastRole === 'tool_result') {
                appendMsg({ tool_result: data.result });
              } else if (lastRole === 'done' || lastRole === 'error') {
                streaming = false;
                sendBtn.disabled = false;
                return;
              }
            } catch (e) {}
          }
        }
      }
    } catch (e) {
      if (e.name === 'AbortError') {
        messages[messages.length - 1].error = 'Request timed out. Is the server running?';
      } else {
        messages[messages.length - 1].error = e.message;
      }
      renderMessages();
    }
    streaming = false;
    sendBtn.disabled = false;
  }

  async function loadFiles(path) {
    var listEl = document.getElementById('file-list');
    var welcomeEl = document.getElementById('welcome-files');
    try {
      var resp = await api('/api/files/list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: path || '.' })
      });
      var data = await resp.json();
      if (!data.success) { welcomeEl.textContent = 'Failed to load files'; return; }
      if (!data.files || data.files.length === 0) {
        welcomeEl.textContent = 'No files found';
        return;
      }
      listEl.innerHTML = '';
      data.files.forEach(function(f) {
        var item = document.createElement('div');
        item.className = 'file-item ' + (f.isDir ? 'folder' : 'file');
        item.textContent = (f.isDir ? '📁 ' : '📄 ') + f.name;
        listEl.appendChild(item);
      });
    } catch (e) {
      welcomeEl.textContent = 'Could not load files';
    }
  }

  async function checkConnection() {
    var statusEl = document.getElementById('conn-status');
    var bannerEl = document.getElementById('conn-banner');
    var welcomeEl = document.getElementById('welcome-files');
    try {
      var resp = await api('/api/ping', { method: 'GET' });
      var data = await resp.json();
      connected = true;
      statusEl.textContent = data.webdav_online ? '🟢 Connected to Filen' : '🟡 Filen WebDAV offline';
      statusEl.className = data.webdav_online ? 'status-ok' : '';
      bannerEl.classList.remove('show');
      welcomeEl.textContent = data.webdav_online ? 'Loading files...' : 'WebDAV offline — chat still works';
      if (data.webdav_online) loadFiles('.');
    } catch (e) {
      connected = false;
      statusEl.textContent = '🔴 Server unreachable';
      statusEl.className = 'status-err';
      bannerEl.classList.add('show');
      welcomeEl.textContent = 'Cannot reach server';
    }
  }

  function autoResize(el) {
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 120) + 'px';
  }

  document.getElementById('messageInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });
  document.getElementById('messageInput').addEventListener('input', function() { autoResize(this); });
  document.getElementById('sendBtn').addEventListener('click', sendMessage);

  checkConnection();
})();
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
