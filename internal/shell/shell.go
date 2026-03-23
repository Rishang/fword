// Package shell generates the shell-specific init scripts that hook fword into the terminal.
// Usage: eval "$(fword --shell-init bash)"  or  eval "$(fword --shell-init zsh)"
package shell

import "fmt"

// Init returns the shell snippet for the given shell name.
// The snippet:
//  1. Hooks into the shell's pre-command mechanism to record exit code + last command
//  2. Defines a `fword` function that invokes the real binary
func Init(shell, binaryPath string) (string, error) {
	switch shell {
	case "bash":
		return bashInit(binaryPath), nil
	case "zsh":
		return zshInit(binaryPath), nil
	case "fish":
		return fishInit(binaryPath), nil
	default:
		return "", fmt.Errorf("unsupported shell %q — supported: bash, zsh, fish", shell)
	}
}

// bashInit emits PROMPT_COMMAND-based hooks for bash
func bashInit(bin string) string {
	return fmt.Sprintf(`
# fword shell integration — added by: fword --shell-init bash
# Paste this into ~/.bashrc or run: eval "$(fword --shell-init bash)"

_fword_hook() {
  # Capture exit code immediately before anything else runs
  export FWORD_EXIT_CODE=$?
  # Grab last history entry, strip the history number prefix
  export FWORD_LAST_CMD=$(HISTTIMEFORMAT="" history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
}

# Prepend to PROMPT_COMMAND so it fires before any other hooks
if [[ "$PROMPT_COMMAND" != *"_fword_hook"* ]]; then
  PROMPT_COMMAND="_fword_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi

# fword shell function — invokes the binary directly
fword() {
  %s "$@"
}
`, bin)
}

// zshInit emits precmd-based hooks for zsh
func zshInit(bin string) string {
	return fmt.Sprintf(`
# fword shell integration — added by: fword --shell-init zsh
# Paste this into ~/.zshrc or run: eval "$(fword --shell-init zsh)"

_fword_hook() {
  export FWORD_EXIT_CODE=$?
  # fc -ln -1 prints last command without line number
  export FWORD_LAST_CMD=$(fc -ln -1 2>/dev/null | sed 's/^[[:space:]]*//')
}

# Register with zsh's precmd array (runs after every command, before prompt)
autoload -Uz add-zsh-hook
add-zsh-hook precmd _fword_hook

# fword shell function
fword() {
  %s "$@"
}
`, bin)
}

// fishInit emits event-based hooks for fish shell
func fishInit(bin string) string {
	return fmt.Sprintf(`
# fword shell integration — fish
# Add to ~/.config/fish/config.fish or run: fword --shell-init fish | source

function _fword_hook --on-event fish_postexec
  set -gx FWORD_EXIT_CODE $status
  set -gx FWORD_LAST_CMD $argv[1]
end

function fword
  %s $argv
end
`, bin)
}
