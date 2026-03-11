package main

import (
"flag"
"fmt"
"os"

"github.com/t0mtait/gofilen/internal/config"
"github.com/t0mtait/gofilen/internal/tui"
)

func main() {
cfg := config.Default()

flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "Filen mount directory")
flag.StringVar(&cfg.Model, "model", cfg.Model, "Ollama model name")
flag.StringVar(&cfg.OllamaURL, "ollama", cfg.OllamaURL, "Ollama API base URL")
flag.Parse()

if err := tui.Run(cfg); err != nil {
fmt.Fprintln(os.Stderr, "error:", err)
os.Exit(1)
}
}
