// Package config manages fword configuration stored at ~/.config/fword/config.yaml
// Uses a hand-rolled key: value parser — no external dependencies.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ProviderClaude     = "claude"
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderGemini     = "gemini"
)

// Config holds all fword configuration options
type Config struct {
	// Provider selects the AI backend: claude, openai, openrouter, gemini
	Provider string

	// APIKey is the authentication token for the chosen provider
	APIKey string

	// BaseURL overrides the default API endpoint (useful for proxies / local models)
	BaseURL string

	// Model selects which model to use (e.g. claude-sonnet-4-20250514, gpt-4o, gemini-1.5-flash)
	Model string

	// AutoRun skips the confirmation prompt and executes the suggested command immediately
	AutoRun bool

	// MaxTokens caps the AI response length (default 512)
	MaxTokens int
}

// Defaults returns sensible out-of-the-box values
func Defaults() *Config {
	return &Config{
		Provider:  ProviderClaude,
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 512,
	}
}

// Path returns the canonical config file location
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "fword", "config.yaml")
}

// Load reads config from disk; missing file returns defaults without error
func Load() (*Config, error) {
	cfg := Defaults()
	path := Path()

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil // first run — defaults are fine
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := parseLine(line)
		if !ok {
			continue
		}
		_ = cfg.set(key, val) // unknown keys silently ignored for forward-compat
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 512
	}
	return cfg, scanner.Err()
}

// Save writes config back to disk as simple key: value YAML
func Save(cfg *Config) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# fword configuration\n")
	sb.WriteString("# Edit manually or use: fword config set <key> <value>\n\n")

	writeField := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}
	writeField("provider", cfg.Provider)
	writeField("api_key", cfg.APIKey)
	writeField("model", cfg.Model)
	writeField("base_url", cfg.BaseURL)
	fmt.Fprintf(&sb, "auto_run: %v\n", cfg.AutoRun)
	fmt.Fprintf(&sb, "max_tokens: %d\n", cfg.MaxTokens)

	return os.WriteFile(path, []byte(sb.String()), 0600)
}

// Validate returns an error if required fields are missing
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key is not set — run: fword config set api_key <YOUR_KEY>")
	}
	if c.Provider == "" {
		return fmt.Errorf("provider is not set — run: fword config set provider <claude|openai|openrouter|gemini>")
	}
	if c.Model == "" {
		return fmt.Errorf("model is not set — run: fword config set model <MODEL_NAME>")
	}
	return nil
}

// DefaultBaseURL returns the canonical API URL for each provider
func (c *Config) DefaultBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	switch c.Provider {
	case ProviderClaude:
		return "https://api.anthropic.com"
	case ProviderOpenAI:
		return "https://api.openai.com"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api"
	case ProviderGemini:
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}

// set applies a single key=value pair to the config struct
func (c *Config) set(key, val string) error {
	switch key {
	case "provider":
		c.Provider = val
	case "api_key":
		c.APIKey = val
	case "model":
		c.Model = val
	case "base_url":
		c.BaseURL = val
	case "auto_run":
		c.AutoRun = val == "true" || val == "1" || val == "yes"
	case "max_tokens":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("max_tokens must be a positive integer")
		}
		c.MaxTokens = n
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
	return nil
}

// Set is the public version of set, used by config subcommands
func (c *Config) Set(key, val string) error {
	return c.set(key, val)
}

// parseLine splits "key: value" into (key, value, true)
func parseLine(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	// Strip optional inline quotes
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	return key, val, key != ""
}
