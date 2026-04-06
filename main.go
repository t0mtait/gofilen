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

	flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "Filen mount directory")
	flag.StringVar(&cfg.Model, "model", cfg.Model, "Ollama model name")
	flag.StringVar(&cfg.OllamaURL, "ollama", cfg.OllamaURL, "Ollama API base URL")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("gofilen", version)
		return
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
