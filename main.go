package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/t0mtait/gofilen/internal/config"
	"github.com/t0mtait/gofilen/internal/tui"
)

var version = "dev"

func main() {
	cfg := config.Default()

	serverMode := flag.Bool("server", false, "Run as HTTP server (web UI)")

	// TUI flags
	flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "Filen mount directory")
	flag.StringVar(&cfg.Model, "model", cfg.Model, "Ollama model name")
	flag.StringVar(&cfg.OllamaURL, "ollama", cfg.OllamaURL, "Ollama API base URL")

	// Server flags
	flag.StringVar(&cfg.WebDAVURL, "webdav-url", cfg.WebDAVURL, "WebDAV server URL")
	flag.StringVar(&cfg.WebDAVUser, "webdav-user", cfg.WebDAVUser, "WebDAV username")
	flag.StringVar(&cfg.WebDAVPassword, "webdav-password", cfg.WebDAVPassword, "WebDAV password")
	flag.StringVar(&cfg.ServerPort, "port", cfg.ServerPort, "HTTP server port")
	flag.StringVar(&cfg.FilenDataDir, "filen-data-dir", cfg.FilenDataDir, "Filen CLI data directory")

	versionFlag := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("gofilen", version)
		return
	}

	if *serverMode {
		// Run as HTTP server - delegate to cmd/server
		runServer(cfg)
		return
	}

	// Run TUI (original behavior)
	if err := tui.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runServer(cfg config.Config) {
	// Server mode requires the cmd/server entry point for proper initialization.
	// Running the server from the main binary is not supported.
	fmt.Fprintln(os.Stderr, "Error: server mode must be run via the cmd/server package:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/server \\")
	fmt.Fprintln(os.Stderr, "    --server \\")
	fmt.Fprintln(os.Stderr, "    --model", cfg.Model)
	fmt.Fprintln(os.Stderr, "    --ollama", cfg.OllamaURL)
	fmt.Fprintln(os.Stderr, "    --webdav-url", cfg.WebDAVURL)
	fmt.Fprintln(os.Stderr, "    --webdav-user", cfg.WebDAVUser)
	fmt.Fprintln(os.Stderr, "    --port", cfg.ServerPort)
	os.Exit(1)
}
