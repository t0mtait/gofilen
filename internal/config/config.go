package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Dir            string // Filen mount directory (local mode)
	Model          string // Ollama model name
	OllamaURL      string // Ollama API base URL
	WebDAVURL      string // WebDAV server URL (default "http://localhost:8080")
	WebDAVUser     string // WebDAV username
	WebDAVPassword string // WebDAV password
	ServerPort     string // HTTP server port (default "3001")
	FilenDataDir   string // Filen CLI data dir (for webdav server)
}

func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return Config{
		Dir:          filepath.Join(home, "filen"),
		Model:        "llama3.2",
		OllamaURL:    "http://localhost:11434",
		WebDAVURL:    "http://localhost:8080",
		ServerPort:   "3001",
		FilenDataDir: filepath.Join(home, ".filen-cli"),
	}
}

// StartWebDAVServer starts the Filen WebDAV server as a background process.
// It waits until the server is ready by polling http://localhost:8080.
func (c *Config) StartWebDAVServer() error {
	if c.WebDAVUser == "" || c.WebDAVPassword == "" {
		return fmt.Errorf("WebDAV credentials not configured")
	}

	// Check if already running
	resp, err := httpGet(c.WebDAVURL)
	if err == nil && resp != nil {
		resp.Body.Close()
		return nil
	}

	// Find the filen binary
	filenPath, err := exec.LookPath("filen")
	if err != nil {
		return fmt.Errorf("filen CLI not found in PATH: %w", err)
	}

	args := []string{
		"webdav",
		"--w-user", c.WebDAVUser,
		"--w-password", c.WebDAVPassword,
		"--w-port", "8080",
	}
	if c.FilenDataDir != "" {
		args = append(args, "--data-dir", c.FilenDataDir)
	}

	cmd := exec.Command(filenPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start filen webdav: %w", err)
	}

	// Wait for server to be ready
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpGet(c.WebDAVURL)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for WebDAV server to start")
}

func httpGet(url string) (*http.Response, error) {
	return http.Get(url)
}

// SaveToFile saves configuration to a JSON file.
func (c *Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadFromFile loads configuration from a JSON file.
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load loads configuration from the default config file path.
func (c *Config) Load() error {
	path := configPath()
	cfg, err := LoadFromFile(path)
	if err != nil {
		return err
	}
	*c = cfg
	return nil
}

// Save saves configuration to the default config file path.
func (c *Config) Save() error {
	path := configPath()
	return c.SaveToFile(path)
}

func configPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "gofilen", "config.json")
}

// Merge updates c with values from another Config, using non-empty values.
func (c *Config) Merge(other Config) {
	if other.Dir != "" {
		c.Dir = other.Dir
	}
	if other.Model != "" {
		c.Model = other.Model
	}
	if other.OllamaURL != "" {
		c.OllamaURL = other.OllamaURL
	}
	if other.WebDAVURL != "" {
		c.WebDAVURL = other.WebDAVURL
	}
	if other.WebDAVUser != "" {
		c.WebDAVUser = other.WebDAVUser
	}
	if other.WebDAVPassword != "" {
		c.WebDAVPassword = other.WebDAVPassword
	}
	if other.ServerPort != "" {
		c.ServerPort = other.ServerPort
	}
	if other.FilenDataDir != "" {
		c.FilenDataDir = other.FilenDataDir
	}
}

// HasWebDAVCredentials returns true if WebDAV credentials are configured.
func (c *Config) HasWebDAVCredentials() bool {
	return c.WebDAVUser != "" && c.WebDAVPassword != ""
}

// ParseConfigFromFlags parses command-line flags into a Config.
// It takes the current Config as defaults and updates with flag values.
func ParseConfigFromFlags(cfg Config, dir, model, ollama, webdavURL, webdavUser, webdavPassword, serverPort, filenDataDir *string) Config {
	result := cfg
	if dir != nil && *dir != "" {
		result.Dir = *dir
	}
	if model != nil && *model != "" {
		result.Model = *model
	}
	if ollama != nil && *ollama != "" {
		result.OllamaURL = *ollama
	}
	if webdavURL != nil && *webdavURL != "" {
		result.WebDAVURL = *webdavURL
	}
	if webdavUser != nil && *webdavUser != "" {
		result.WebDAVUser = *webdavUser
	}
	if webdavPassword != nil && *webdavPassword != "" {
		result.WebDAVPassword = *webdavPassword
	}
	if serverPort != nil && *serverPort != "" {
		result.ServerPort = *serverPort
	}
	if filenDataDir != nil && *filenDataDir != "" {
		result.FilenDataDir = *filenDataDir
	}
	return result
}

// Validate returns an error if the config is invalid for server mode.
func (c *Config) Validate() []string {
	var errs []string
	if c.Model == "" {
		errs = append(errs, "model is required")
	}
	if c.OllamaURL == "" {
		errs = append(errs, "ollama_url is required")
	}
	if c.ServerPort == "" {
		errs = append(errs, "server_port is required")
	}
	return errs
}

// ToMap returns the config as a map (useful for JSON responses).
func (c *Config) ToMap() map[string]string {
	return map[string]string{
		"model":       c.Model,
		"ollama_url":  c.OllamaURL,
		"webdav_url":  c.WebDAVURL,
		"server_port": c.ServerPort,
	}
}

// Redact returns a copy of the config with sensitive fields redacted.
func (c *Config) Redact() Config {
	result := *c
	result.WebDAVPassword = redactString(c.WebDAVPassword)
	return result
}

func redactString(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
