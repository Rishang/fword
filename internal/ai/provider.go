// Package ai defines the Provider interface and factory for all AI backends
package ai

import (
	"context"
	"fmt"

	"github.com/user/fword/internal/config"
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

// userPrompt builds the per-request message
func UserPrompt(r *Request) string {
	msg := fmt.Sprintf("Command: %s\nExit code: %d", r.Command, r.ExitCode)
	if r.Output != "" {
		// Truncate very long output to avoid bloating the prompt
		out := r.Output
		if len(out) > 2000 {
			out = out[:2000] + "\n... (truncated)"
		}
		msg += fmt.Sprintf("\nOutput:\n%s", out)
	}
	if r.Shell != "" {
		msg += fmt.Sprintf("\nShell: %s", r.Shell)
	}
	return msg
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

// ParseSuggestion converts raw AI text into a structured Suggestion
func ParseSuggestion(raw string) *Suggestion {
	s := &Suggestion{Raw: raw}

	// Scan for "FIX: <cmd>" pattern (single corrected command)
	if after, ok := cutPrefix(raw, "FIX:"); ok {
		s.Kind = "fix"
		s.Commands = []string{trimSpace(after)}
		return s
	}

	// Scan for "STEPS:" followed by "$ <cmd>" lines
	if _, ok := cutPrefix(raw, "STEPS:"); ok {
		s.Kind = "steps"
		for _, line := range splitLines(raw) {
			if cmd, ok := cutPrefix(line, "$"); ok {
				if c := trimSpace(cmd); c != "" {
					s.Commands = append(s.Commands, c)
				}
			}
		}
		if len(s.Commands) > 0 {
			return s
		}
	}

	// Fallback: treat the whole response as raw steps to display
	s.Kind = "raw"
	s.Commands = []string{raw}
	return s
}

// ---- tiny helpers to avoid importing strings in every file ----

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return s, false
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
