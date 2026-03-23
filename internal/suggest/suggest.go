// Package suggest handles rendering the AI suggestion and optionally running it
package suggest

import (
	"fmt"
	"os"

	"github.com/user/fword/internal/ai"
)

// ANSI color codes — kept as constants so the whole file stays dependency-free
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBgDark = "\033[48;5;235m"
)

// Present prints suggestions in plain text only.
func Present(s *ai.Suggestion, _ string, _ bool) bool {
	switch s.Kind {
	case "fix":
		if len(s.Commands) > 0 {
			fmt.Println(s.Commands[0])
		}
	case "steps":
		for _, cmd := range s.Commands {
			fmt.Println(cmd)
		}
	default:
		fmt.Println(s.Raw)
	}
	return false
}

// PrintError prints a styled error to stderr
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "\n%s%s✗ fword: %s%s\n\n", colorBold, colorRed, colorReset, msg)
}

// PrintSpinner is intentionally a no-op to keep output plain.
func PrintSpinner(provider, model string) func() {
	_ = provider
	_ = model
	return func() {}
}
