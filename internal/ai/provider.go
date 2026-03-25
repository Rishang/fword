// Package ai defines the Provider interface and factory for all AI backends
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Rishang/fword/internal/config"
)

// Suggestion is the structured response from any AI provider
type Suggestion struct {
	// Kind is either "fix" (single command) or "steps" (multi-step troubleshoot)
	Kind string

	// Commands holds one corrected command ("fix") or ordered troubleshoot steps ("steps")
	Commands []string

	// Raw is the full unprocessed AI response for debug/fallback display
	Raw string
}

// Provider is the interface every AI backend must satisfy
type Provider interface {
	// Query sends the failed-command context to the AI and returns a parsed Suggestion
	Query(ctx context.Context, req *Request) (*Suggestion, error)
}

// Request bundles everything we know about the failed command
type Request struct {
	// Command is the raw text the user typed (e.g. "git psuh origin main")
	Command string

	// ExitCode is the non-zero return code from the shell
	ExitCode int

	// Output is optional captured stderr/stdout from re-running the command
	Output string

	// Shell is the detected shell name (bash, zsh, fish) for context
	Shell string
}

// systemPrompt is injected into every provider call to constrain output format
const systemPrompt = `You are an expert command-line troubleshooter.
The user ran a command that failed. Your job: give them the fix — nothing else.

Rules:
- Output ONLY commands the user should run. No prose. No explanations. No markdown fences.
- If one corrected command fixes it, output exactly: FIX: <command>
- If multiple steps are needed, output exactly:
  STEPS:
  $ <step1>
  $ <step2>
  ...
- Prefer the shortest, most direct path to success.
- Never suggest installing docs readers, man pages, or external resources.
- Commands only. Every line must be something the user can paste into a terminal.`

// UserPrompt builds the per-request message sent to every provider.
func UserPrompt(r *Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Command: %s\nExit code: %d", r.Command, r.ExitCode)
	if r.Output != "" {
		out := r.Output
		if len(out) > 2000 {
			out = out[:2000] + "\n... (truncated)"
		}
		fmt.Fprintf(&b, "\nOutput:\n%s", out)
	}
	if r.Shell != "" {
		fmt.Fprintf(&b, "\nShell: %s", r.Shell)
	}
	return b.String()
}

// SystemPrompt exposes the constant for providers that need it
func SystemPrompt() string { return systemPrompt }

// New returns the correct Provider implementation for the given config
func New(cfg *config.Config) (Provider, error) {
	switch cfg.Provider {
	case config.ProviderClaude:
		return NewClaude(cfg), nil
	case config.ProviderOpenAI, config.ProviderOpenRouter:
		// OpenRouter is OpenAI-compatible — same client, different base URL
		return NewOpenAI(cfg), nil
	case config.ProviderGemini:
		return NewGemini(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider %q — choose: claude, openai, openrouter, gemini", cfg.Provider)
	}
}

// doJSON marshals reqBody as JSON, POSTs to url with the given headers, reads the
// response body, checks for a non-200 status, then JSON-decodes into respBody.
// It is the single HTTP round-trip used by every provider.
func doJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, reqBody, respBody any) ([]byte, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, data)
	}
	if err := json.Unmarshal(data, respBody); err != nil {
		return data, fmt.Errorf("parse response: %w", err)
	}
	return data, nil
}

// ParseSuggestion converts raw AI text into a structured Suggestion.
func ParseSuggestion(raw string) *Suggestion {
	s := &Suggestion{Raw: raw}

	if after, ok := strings.CutPrefix(raw, "FIX:"); ok {
		s.Kind = "fix"
		s.Commands = []string{strings.TrimSpace(after)}
		return s
	}

	if strings.HasPrefix(raw, "STEPS:") {
		s.Kind = "steps"
		for _, line := range strings.Split(raw, "\n") {
			if after, ok := strings.CutPrefix(strings.TrimSpace(line), "$"); ok {
				if cmd := strings.TrimSpace(after); cmd != "" {
					s.Commands = append(s.Commands, cmd)
				}
			}
		}
		if len(s.Commands) > 0 {
			return s
		}
	}

	s.Kind = "raw"
	s.Commands = []string{raw}
	return s
}
