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

	"github.com/Rishang/fk/internal/ai"
	"github.com/Rishang/fk/internal/config"
	"github.com/Rishang/fk/internal/fs"
	"github.com/Rishang/fk/internal/logger"
	"github.com/Rishang/fk/internal/shell"
	"github.com/Rishang/fk/internal/suggest"
	cli "github.com/urfave/cli/v3"
)

// version is set at build time via: go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	if err := run(os.Args); err != nil {
		suggest.PrintError(err.Error())
		os.Exit(1)
	}
}

// run builds the CLI command tree and dispatches to the appropriate action.
// showCfg is shared between `config` and `config show` to avoid duplication.
func run(args []string) error {
	showCfg := func(_ context.Context, _ *cli.Command) error { return configShow() }
	cli.VersionFlag = &cli.BoolFlag{Name: "version", Usage: "print the version"}
	app := &cli.Command{
		Name: "fk", Usage: "AI-powered shell command corrector", Version: version,
		Action: runMain,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "shell-init", Usage: "Print shell integration snippet (bash|zsh|fish)"},
			&cli.StringFlag{Name: "cmd", Usage: "The failed command text"},
			&cli.IntFlag{Name: "exit-code", Value: -1, Usage: "The exit code from the failed command"},
			&cli.StringFlag{Name: "output", Usage: "Captured stdout/stderr from the failed command"},
			&cli.BoolFlag{Name: "rerun", Aliases: []string{"r"}, Usage: "Re-run failed command to capture output (side effects possible)"},
			&cli.StringFlag{Name: "shell", Usage: "Override shell detection (bash|zsh|fish)"},
			&cli.BoolFlag{Name: "auto-run", Usage: "Execute suggestion without confirmation prompt"},
			&cli.BoolFlag{Name: "debug", Usage: "Print raw AI response before parsing"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Print prompts sent to LLM and token estimates"},
		},
		Commands: []*cli.Command{
			{
				Name:      "cat",
				Usage:     "Format files/directory as a prompt-ready string",
				ArgsUsage: "[file|dir ...]",
				Action: func(_ context.Context, c *cli.Command) error {
					return runCat(c)
				},
			},
			{
				Name: "config", Usage: "Show or update fk config", Action: showCfg,
				Commands: []*cli.Command{
					{Name: "show", Usage: "Print current config", Action: showCfg},
					{Name: "set", Usage: "Update config values", ArgsUsage: "<key> <value> | [--provider ... --api-token ...]",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "provider"}, &cli.StringFlag{Name: "api-token", Aliases: []string{"api-key"}},
							&cli.StringFlag{Name: "model"}, &cli.StringFlag{Name: "base-url"},
							&cli.BoolFlag{Name: "auto-run"}, &cli.IntFlag{Name: "max-tokens"},
						},
						// Supports both positional (fk config set key val)
						// and flag form (fk config set --provider claude --api-token sk-...).
						Action: func(_ context.Context, c *cli.Command) error {
							if c.Args().Len() >= 2 {
								return configSet(c.Args().Get(0), c.Args().Get(1))
							}
							cfg, err := config.Load()
							if err != nil {
								return err
							}
							n, err := applyFlagUpdates(c, cfg)
							if err != nil {
								return err
							}
							if n == 0 {
								return fmt.Errorf("usage: fk config set <key> <value> OR fk config set --provider <name> --api-token <token>")
							}
							if err := config.Save(cfg); err != nil {
								return err
							}
							fmt.Printf("\033[32m✓  updated %d config value(s)\033[0m\n", n)
							return nil
						}},
				},
			}},
	}
	// Set debug level before running so all packages respect it.
	for _, arg := range args {
		if arg == "--verbose" || arg == "-v" {
			logger.SetDebug(true)
			break
		}
	}
	return app.Run(context.Background(), args)
}

// runCat is the action for `fk cat`. Each positional arg is either a file
// or a directory; directories are walked recursively (respecting .gitignore).
// When no args are given the current directory is walked.
func runCat(c *cli.Command) error {
	baseDir, _ := os.Getwd()

	args := c.Args().Slice()
	if len(args) == 0 {
		args = []string{"."}
	}

	var paths []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("%s: %w", arg, err)
		}
		if info.IsDir() {
			dirFiles, err := fs.FilesFromDir(arg)
			if err != nil {
				return err
			}
			paths = append(paths, dirFiles...)
		} else {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return err
			}
			paths = append(paths, abs)
		}
	}

	out, err := fs.Format(paths, baseDir)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// runMain is the primary CLI action. It delegates each concern to a focused
// helper: shell detection, shell-init output, command/exit-code resolution,
// config loading, optional output capture, and the AI query + presentation.
func runMain(_ context.Context, c *cli.Command) error {
	shellName := detectShell(c.String("shell"))

	if si := c.String("shell-init"); si != "" {
		return printShellInit(si)
	}

	cmd, err := resolveCmd(shellName)
	if err != nil {
		return err
	}
	if isFkCommand(cmd) {
		fmt.Println("fk can't fk")
		return nil
	}

	exitCode, err := resolveExitCode(c, shellName)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if c.Bool("auto-run") {
		cfg.AutoRun = true // CLI flag overrides persisted config
	}

	output, err := resolveOutput(c, shellName, cmd)
	if err != nil {
		return err
	}

	return queryAndPresent(c, cfg, &ai.Request{
		Command:  cmd,
		ExitCode: exitCode,
		Output:   output,
		Shell:    shellName,
	})
}

// detectShell returns the shell name from the explicit override or $SHELL basename.
func detectShell(override string) string {
	if override != "" {
		return override
	}
	if s := os.Getenv("SHELL"); s != "" {
		parts := strings.Split(s, "/")
		return parts[len(parts)-1]
	}
	return "sh"
}

// printShellInit prints the eval-able shell hook snippet for the given shell and exits.
// The absolute path to the current binary is resolved so the hook calls the right executable.
func printShellInit(shellName string) error {
	bin, err := exec.LookPath(os.Args[0])
	if err != nil {
		bin = os.Args[0] // fall back to argv[0] if not found on PATH
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	snippet, err := shell.Init(shellName, bin)
	if err != nil {
		return err
	}
	fmt.Print(snippet)
	return nil
}

// resolveCmd returns the failed command text.
// Explicit --cmd flag takes priority over the $fk_LAST_CMD env set by the shell hook.
func resolveCmd(shellName string) (string, error) {
	if cmd := strings.TrimSpace(os.Getenv("fk_LAST_CMD")); cmd != "" {
		return cmd, nil
	}
	return "", fmt.Errorf("no command context found — run with --cmd or enable shell integration: eval \"$(fk --shell-init %s)\"", shellName)
}

// isFkCommand reports whether cmd is an invocation of fk (guards --rerun from recursion).
func isFkCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	return filepath.Base(fields[0]) == "fk"
}

// resolveExitCode returns the exit code of the failed command.
// Explicit --exit-code flag takes priority over the $fk_EXIT_CODE env set by the shell hook.
func resolveExitCode(c *cli.Command, shellName string) (int, error) {
	if code := c.Int("exit-code"); code != -1 {
		return code, nil
	}
	if s := strings.TrimSpace(os.Getenv("fk_EXIT_CODE")); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n, nil
		}
	}
	return -1, fmt.Errorf("no exit code context found — run with --exit-code or enable shell integration: eval \"$(fk --shell-init %s)\"", shellName)
}

// resolveOutput returns the captured output to send to the AI.
// If --output was provided it is used directly. If --rerun is set, the failed
// command is re-executed to capture its combined stdout/stderr (side effects possible).
func resolveOutput(c *cli.Command, shellName, cmd string) (string, error) {
	if output := c.String("output"); output != "" {
		return output, nil
	}
	if c.Bool("rerun") {
		if isFkCommand(cmd) {
			return "", nil // refuse to re-run fk itself
		}
		exe, args := shellExecArgs(shellName, cmd)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		out, _ := exec.CommandContext(ctx, exe, args...).CombinedOutput()
		return strings.TrimSpace(string(out)), nil
	}
	return "", nil
}

// queryAndPresent sends the AI request, stops the spinner, and prints the suggestion.
func queryAndPresent(c *cli.Command, cfg *config.Config, req *ai.Request) error {
	provider, err := ai.New(cfg)
	if err != nil {
		return err
	}
	stop := suggest.PrintSpinner(cfg.Provider, cfg.Model)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	s, err := provider.Query(ctx, req)
	stop()
	if err != nil {
		return fmt.Errorf("AI query failed: %w", err)
	}
	if c.Bool("debug") {
		fmt.Printf("\n\033[90m[debug] raw response:\n%s\033[0m\n", s.Raw)
	}
	suggest.Present(s)
	return nil
}

// applyFlagUpdates writes any explicitly set flags from `config set` into cfg.
// Returns the count of fields updated so the caller can detect a no-op invocation.
func applyFlagUpdates(c *cli.Command, cfg *config.Config) (int, error) {
	pairs := []struct {
		set  bool
		k, v string
	}{
		{c.IsSet("provider"), "provider", c.String("provider")},
		{c.IsSet("api-token") || c.IsSet("api-key"), "api_key", c.String("api-token")},
		{c.IsSet("model"), "model", c.String("model")},
		{c.IsSet("base-url"), "base_url", c.String("base-url")},
		{c.IsSet("auto-run"), "auto_run", fmt.Sprintf("%t", c.Bool("auto-run"))},
		{c.IsSet("max-tokens"), "max_tokens", fmt.Sprintf("%d", c.Int("max-tokens"))},
	}
	n := 0
	for _, p := range pairs {
		if p.set {
			if err := cfg.Set(p.k, p.v); err != nil {
				return 0, err
			}
			n++
		}
	}
	return n, nil
}

// configShow prints the current config, masking all but the first/last 4 chars of the API key.
func configShow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	key := cfg.APIKey
	if len(key) > 8 {
		key = key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
	}
	fmt.Printf("\n\033[1mfk config\033[0m  (%s)\n\n", config.Path())
	fmt.Printf("  %-18s %s\n", "provider", cfg.Provider)
	fmt.Printf("  %-18s %s\n", "model", cfg.Model)
	fmt.Printf("  %-18s %s\n", "api_key", key)
	if cfg.BaseURL != "" {
		fmt.Printf("  %-18s %s\n", "base_url", cfg.BaseURL)
	}
	fmt.Printf("  %-18s %v\n", "auto_run", cfg.AutoRun)
	fmt.Printf("  %-18s %d\n\n", "max_tokens", cfg.MaxTokens)
	return nil
}

// configSet updates a single key=value pair in the persisted config file.
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

// shellExecArgs returns the executable and arguments needed to run cmd in the given shell.
// fish uses -c (no login); bash/zsh use -lc (login shell) to load the user's PATH.
func shellExecArgs(sh, cmd string) (string, []string) {
	switch sh {
	case "fish":
		return "fish", []string{"-c", cmd}
	case "bash", "zsh":
		return sh, []string{"-lc", cmd}
	default:
		return "sh", []string{"-lc", cmd}
	}
}
