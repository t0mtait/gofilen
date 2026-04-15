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
	// Import and run the server main
	// This is a bit of a hack - in production you'd build separately
	// For now, we just print an error since cmd/server has its own main
	fmt.Println("To run the server, build and run the cmd/server package:")
	fmt.Println("  go run ./cmd/server --server \\")
	fmt.Println("    --model", cfg.Model)
	fmt.Println("    --ollama", cfg.OllamaURL)
	fmt.Println("    --webdav-url", cfg.WebDAVURL)
	fmt.Println("    --webdav-user", cfg.WebDAVUser)
	fmt.Println("    --webdav-password", cfg.WebDAVPassword)
	fmt.Println("    --port", cfg.ServerPort)
}