package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/t0mtait/gofilen/internal/config"
	"github.com/t0mtait/gofilen/internal/tui"
	"github.com/t0mtait/gofilen/cmd/server"
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
	if err := server.RunServer(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
