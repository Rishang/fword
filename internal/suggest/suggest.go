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

// Present prints the AI suggestion to stdout.
func Present(s *ai.Suggestion) {
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
}

// PrintError prints a styled error to stderr
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "\n%s%s✗ fword: %s%s\n\n", colorBold, colorRed, colorReset, msg)
}

// PrintSpinner is a no-op placeholder; kept for future animated spinner support.
func PrintSpinner(_, _ string) func() { return func() {} }
