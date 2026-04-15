package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/t0mtait/gofilen/internal/config"
	"github.com/t0mtait/gofilen/internal/filer"
	"github.com/t0mtait/gofilen/internal/llm"
)

var version = "dev"

func main() {
	cfg := config.Default()

	serverMode := flag.Bool("server", false, "Run as HTTP server")
	flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "Filen mount directory (local mode)")
	flag.StringVar(&cfg.Model, "model", cfg.Model, "Ollama model name")
	flag.StringVar(&cfg.OllamaURL, "ollama", cfg.OllamaURL, "Ollama API base URL")
	flag.StringVar(&cfg.WebDAVURL, "webdav-url", cfg.WebDAVURL, "WebDAV server URL")
	flag.StringVar(&cfg.WebDAVUser, "webdav-user", cfg.WebDAVUser, "WebDAV username")
	flag.StringVar(&cfg.WebDAVPassword, "webdav-password", cfg.WebDAVPassword, "WebDAV password")
	flag.StringVar(&cfg.ServerPort, "port", cfg.ServerPort, "HTTP server port")
	flag.StringVar(&cfg.FilenDataDir, "filen-data-dir", cfg.FilenDataDir, "Filen CLI data directory")
	versionFlag := flag.Bool("v", false, "Show version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("gofilen-server", version)
		return
	}

	if !*serverMode {
		fmt.Println("Use --server to run as HTTP server")
		fmt.Println("  go run ./cmd/server --server")
		return
	}

	if errs := cfg.Validate(); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "config error: %s\n", err)
		}
		os.Exit(1)
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
		log.Fatalf("Server failed: %v", err)
	}
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
		result, err := f.List(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return nil
		}
		writeJSON(w, map[string]string{"result": result})
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
		json.NewDecoder(r.Body).Decode(&req)
		if req.Depth < 1 || req.Depth > 10 {
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
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; height: 100vh; overflow: hidden; }
    #root { height: 100%; }
  </style>
</head>
<body>
  <div id="root">
    <div style="display: flex; align-items: center; justify-content: center; height: 100vh; flex-direction: column; gap: 16px;">
      <p style="color: #64748b;">Loading gofilen...</p>
    </div>
  </div>
  <script>
    const API_BASE = '';
    let messages = [];
    let streaming = false;

    async function api(path, opts = {}) {
      const res = await fetch(API_BASE + path, {
        headers: { 'Content-Type': 'application/json' },
        ...opts
      });
      return res.json().catch(() => ({}));
    }

    function escapeHtml(s) {
      if (!s) return '';
      return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>');
    }

    function renderMessages() {
      const container = document.getElementById('messages');
      if (!container) return;
      container.innerHTML = messages.map((m, i) => {
        if (m.tool_call) {
          return '<div style="background: #1e293b; border-radius: 8px; padding: 12px; margin: 8px 0; border-left: 3px solid #3b82f6;">' +
            '<div style="font-size: 12px; color: #3b82f6; margin-bottom: 4px;">🔧 TOOL: ' + escapeHtml(m.tool_call) + '</div>' +
            '<div style="font-size: 12px; color: #94a3b8;">Args: ' + escapeHtml(m.tool_args) + '</div>' +
            '</div>';
        }
        if (m.tool_result) {
          return '<div style="background: #1e293b; border-radius: 8px; padding: 12px; margin: 8px 0; border-left: 3px solid #22c55e;">' +
            '<div style="font-size: 12px; color: #22c55e; margin-bottom: 4px;">✅ RESULT</div>' +
            '<div style="font-size: 12px; color: #94a3b8; white-space: pre-wrap; max-height: 200px; overflow-y: auto;">' + escapeHtml(m.tool_result) + '</div>' +
            '</div>';
        }
        const isUser = m.role === 'user';
        return '<div style="padding: 12px 16px; border-radius: 8px; margin: 8px 0; ' +
          (isUser ? 'background: #1e3a5f; text-align: right; margin-left: 60px;' : 'background: #1e293b; margin-right: 60px;') + '">' +
          '<div style="font-size: 11px; color: #64748b; margin-bottom: 4px;">' + (isUser ? 'You' : 'AI') + '</div>' +
          '<div style="white-space: pre-wrap;">' + escapeHtml(m.content) + '</div>' +
          '</div>';
      }).join('');
      container.scrollTop = container.scrollHeight;
    }

    async function sendMessage() {
      const input = document.getElementById('messageInput');
      const content = input.value.trim();
      if (!content || streaming) return;
      input.value = '';
      messages.push({ role: 'user', content });
      renderMessages();
      streaming = true;

      try {
        messages.push({ role: 'assistant', content: '' });
        renderMessages();

        const resp = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: content, history: [] })
        });

        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let lastRole = 'assistant';
        let lastContent = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          const text = decoder.decode(value);
          const lines = text.split('\\n');
          for (const line of lines) {
            if (line.startsWith('event: ')) {
              lastRole = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              try {
                const data = JSON.parse(line.slice(6));
                if (lastRole === 'chunk' && data.content !== undefined) {
                  lastContent += data.content;
                  messages[messages.length - 1].content = lastContent;
                  renderMessages();
                } else if (lastRole === 'tool_call') {
                  messages.push({ tool_call: data.name, tool_args: data.args });
                  renderMessages();
                } else if (lastRole === 'tool_result') {
                  messages.push({ tool_result: data.result });
                  renderMessages();
                } else if (lastRole === 'done' || lastRole === 'error') {
                  streaming = false;
                  return;
                }
              } catch(e) {}
            }
          }
        }
      } catch (e) {
        messages.push({ role: 'error', content: 'Error: ' + e.message });
      }
      streaming = false;
      renderMessages();
    }

    document.addEventListener('DOMContentLoaded', () => {
      const input = document.getElementById('messageInput');
      if (input) {
        input.addEventListener('keydown', e => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
          }
        });
      }
    });
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}