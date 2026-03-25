package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/user/fword/internal/ai"
	"github.com/user/fword/internal/config"
	"github.com/user/fword/internal/shell"
	"github.com/user/fword/internal/suggest"
)

// version is set at build time via: go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	if err := run(os.Args); err != nil {
		suggest.PrintError(err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	app := &cli.App{
		Name:    "fword",
		Usage:   "AI-powered shell command corrector",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "shell-init", Usage: "Print shell integration snippet (bash|zsh|fish)"},
			&cli.StringFlag{Name: "cmd", Usage: "The failed command text"},
			&cli.IntFlag{Name: "exit-code", Value: -1, Usage: "The exit code from the failed command"},
			&cli.StringFlag{Name: "output", Usage: "Captured stdout/stderr from the failed command"},
			&cli.BoolFlag{Name: "rerun", Aliases: []string{"r"}, Usage: "Re-run failed command to capture output (side effects possible)"},
			&cli.StringFlag{Name: "shell", Usage: "Override shell detection (bash|zsh|fish)"},
			&cli.BoolFlag{Name: "auto-run", Usage: "Execute suggestion without confirmation prompt"},
			&cli.BoolFlag{Name: "debug", Usage: "Print raw AI response before parsing"},
		},
		Action: runMain,
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Show or update fword config",
				Subcommands: []*cli.Command{
					{
						Name:   "show",
						Usage:  "Print current config",
						Action: runConfigShow,
					},
					{
						Name:      "set",
						Usage:     "Update config values",
						ArgsUsage: "<key> <value> | [--provider ... --api-token ...]",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "provider", Usage: "Provider: claude|openai|openrouter|gemini"},
							&cli.StringFlag{Name: "api-token", Aliases: []string{"api-key"}, Usage: "API token/key"},
							&cli.StringFlag{Name: "model", Usage: "Model name"},
							&cli.StringFlag{Name: "base-url", Usage: "Override provider API base URL"},
							&cli.BoolFlag{Name: "auto-run", Usage: "Run suggested command without confirmation"},
							&cli.IntFlag{Name: "max-tokens", Usage: "Max response tokens"},
						},
						Action: runConfigSet,
					},
				},
				Action: runConfigShow,
			},
		},
		// Keep legacy custom help text for backward compatibility.
		CustomAppHelpTemplate: "",
	}

	return app.Run(args)
}

func runMain(c *cli.Context) error {
	if c.String("shell-init") != "" {
		// Find absolute path to current binary so the shell function calls the right one
		bin, err := exec.LookPath(os.Args[0])
		if err != nil {
			bin = os.Args[0]
		}
		bin, _ = filepath.Abs(bin)

		snippet, err := shell.Init(c.String("shell-init"), bin)
		if err != nil {
			return err
		}
		fmt.Print(snippet)
		return nil
	}

	// Detect shell context.
	shellName := c.String("shell")
	if shellName == "" {
		shellName = detectShell()
	}

	cmd := c.String("cmd")
	if cmd == "" {
		cmd = strings.TrimSpace(os.Getenv("FWORD_LAST_CMD"))
	}
	if cmd == "" {
		return fmt.Errorf("no command context found — run with --cmd or enable shell integration: eval \"$(fword --shell-init %s)\"", shellName)
	}

	// Exit code -1 means unknown; prefer explicit flag, then shell hook env.
	exitCode := c.Int("exit-code")
	if exitCode == -1 {
		if s := strings.TrimSpace(os.Getenv("FWORD_EXIT_CODE")); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				exitCode = n
			}
		}
	}
	if exitCode == -1 {
		return fmt.Errorf("no exit code context found — run with --exit-code or enable shell integration: eval \"$(fword --shell-init %s)\"", shellName)
	}
	// Load and validate config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Override auto_run from flag if explicitly passed
	if c.Bool("auto-run") {
		cfg.AutoRun = true
	}

	output := c.String("output")
	if output == "" && c.Bool("rerun") {
		// Best-effort: capture command stderr/stdout for richer AI context.
		captured, _ := captureCommandOutput(shellName, cmd)
		output = strings.TrimSpace(captured)
	}

	// Build AI request
	req := &ai.Request{
		Command:  cmd,
		ExitCode: exitCode,
		Output:   output,
		Shell:    shellName,
	}

	// Create provider + start spinner
	provider, err := ai.New(cfg)
	if err != nil {
		return err
	}

	stopSpinner := suggest.PrintSpinner(cfg.Provider, cfg.Model)

	// Keep CLI responsive by failing slow provider calls sooner.
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	s, err := provider.Query(ctx, req)
	stopSpinner()

	if err != nil {
		return fmt.Errorf("AI query failed: %w", err)
	}

	// Debug mode: show raw response before parsing
	if c.Bool("debug") {
		fmt.Printf("\n\033[90m[debug] raw response:\n%s\033[0m\n", s.Raw)
	}

	suggest.Present(s)
	return nil
}

func runConfigShow(_ *cli.Context) error {
	return configShow()
}

func runConfigSet(c *cli.Context) error {
	// Backward-compatible positional form: fword config set <key> <value>
	if c.Args().Len() >= 2 {
		return configSet(c.Args().Get(0), c.Args().Get(1))
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	updated := 0

	if c.IsSet("provider") {
		if err := cfg.Set("provider", c.String("provider")); err != nil {
			return err
		}
		updated++
	}
	if c.IsSet("api-token") || c.IsSet("api-key") {
		if err := cfg.Set("api_key", c.String("api-token")); err != nil {
			return err
		}
		updated++
	}
	if c.IsSet("model") {
		if err := cfg.Set("model", c.String("model")); err != nil {
			return err
		}
		updated++
	}
	if c.IsSet("base-url") {
		if err := cfg.Set("base_url", c.String("base-url")); err != nil {
			return err
		}
		updated++
	}
	if c.IsSet("auto-run") {
		if err := cfg.Set("auto_run", fmt.Sprintf("%t", c.Bool("auto-run"))); err != nil {
			return err
		}
		updated++
	}
	if c.IsSet("max-tokens") {
		if err := cfg.Set("max_tokens", fmt.Sprintf("%d", c.Int("max-tokens"))); err != nil {
			return err
		}
		updated++
	}

	if updated == 0 {
		return fmt.Errorf("usage: fword config set <key> <value> OR fword config set --provider <name> --api-token <token>")
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("\033[32m✓  updated %d config value(s)\033[0m\n", updated)
	return nil
}

// configShow prints the current config (masking the API key)
func configShow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	maskedKey := cfg.APIKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + strings.Repeat("*", len(maskedKey)-8) + maskedKey[len(maskedKey)-4:]
	}

	fmt.Printf("\n\033[1mfword config\033[0m  (%s)\n\n", config.Path())
	fmt.Printf("  %-18s %s\n", "provider", cfg.Provider)
	fmt.Printf("  %-18s %s\n", "model", cfg.Model)
	fmt.Printf("  %-18s %s\n", "api_key", maskedKey)
	if cfg.BaseURL != "" {
		fmt.Printf("  %-18s %s\n", "base_url", cfg.BaseURL)
	}
	fmt.Printf("  %-18s %v\n", "auto_run", cfg.AutoRun)
	fmt.Printf("  %-18s %d\n", "max_tokens", cfg.MaxTokens)
	fmt.Println()
	return nil
}

// configSet updates a single key in the config file
func configSet(key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := cfg.Set(key, value); err != nil {
		return fmt.Errorf("%s\n  valid keys: provider, api_key, model, base_url, auto_run, max_tokens", err)
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("\033[32m✓  %s = %s\033[0m\n", key, value)
	return nil
}

// detectShell infers the current shell from $SHELL
func detectShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		parts := strings.Split(s, "/")
		return parts[len(parts)-1]
	}
	return "sh"
}

func captureCommandOutput(shellName, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	exe, args := shellExecArgs(shellName, command)
	cmd := exec.CommandContext(ctx, exe, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func shellExecArgs(shellName, command string) (string, []string) {
	switch shellName {
	case "bash", "zsh":
		return shellName, []string{"-lc", command}
	case "fish":
		return "fish", []string{"-c", command}
	default:
		return "sh", []string{"-lc", command}
	}
}

func printUsage() {
	fmt.Print(`
fword — AI shell command corrector

USAGE:
  fword [flags]
  fword config show
  fword config set <key> <value>
  fword config set --provider <name> --api-token <token> [--base-url <url>]

EXAMPLES:
  fword
  fword --cmd "git psuh" --exit-code 1
  fword --cmd "docker ps" --exit-code 1 --output "permission denied" --auto-run
  fword --shell-init zsh

CONFIG KEYS:
  provider | api_key | model | base_url | auto_run | max_tokens

`)
}
