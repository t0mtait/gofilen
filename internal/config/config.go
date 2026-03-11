package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Dir       string // Filen mount directory
	Model     string // Ollama model name
	OllamaURL string // Ollama API base URL
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Dir:       filepath.Join(home, "filen"),
		Model:     "llama3.2",
		OllamaURL: "http://localhost:11434",
	}
}
