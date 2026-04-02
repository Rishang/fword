# fword

AI-powered shell command corrector. After you run a command, `fword` asks an AI model for a fix and prints corrected command(s) as plain text.

Inspired by [thefuck](https://github.com/nvbn/thefuck). Built in Go. Zero runtime dependencies.

```
$ git psuh origin main
fatal: 'psuh' is not a git command.

$ fword
git push origin main
```

---

## Install

### With [install-release](https://pypi.org/project/install-release/) (`ir`)

[install-release](https://github.com/Rishang/install-release) installs release binaries from GitHub for your OS and CPU. The CLI command is **`ir`**.

Prerequisites: Python 3.9+, `pip`, and [libmagic](https://github.com/ahupp/python-magic#installation) (see the install-release readme).

```bash
pip install -U install-release
```

Binaries go to `~/bin` by default — put that on your `PATH` (e.g. in `~/.bashrc` or `~/.zshrc`):

```bash
export PATH="$HOME/bin:$PATH"
```

Install or upgrade **fword** from the latest GitHub release:

```bash
ir get https://github.com/Rishang/fword
```

Useful follow-ups: `ir ls`, `ir upgrade`, `ir rm fword`. Use `ir get --help` for flags (e.g. `-f` to pick a specific release asset).

### Pre-built binary (manual)

Each [release](https://github.com/Rishang/fword/releases/latest) ships plain binaries and a `SHA256SUMS` file. Pick the name that matches your platform:

| Platform   | Asset                 |
|------------|------------------------|
| Linux x64  | `fword-linux-amd64`    |
| Linux arm64| `fword-linux-arm64`    |
| macOS x64  | `fword-darwin-amd64`   |
| macOS arm64| `fword-darwin-arm64`   |

Example (Linux x86_64):

```bash
curl -fL -o fword https://github.com/Rishang/fword/releases/latest/download/fword-linux-amd64
chmod +x fword
sudo mv fword /usr/local/bin/   # or another directory on your PATH
```


### From source (requires Go 1.21+)

```bash
git clone https://github.com/Rishang/fword
cd fword
task install        # installs to $(go env GOPATH)/bin/fword
```

---

## Setup

### 1. Configure your AI provider

```bash
# Claude (Anthropic)
fword config set --provider claude --api-token sk-ant-xxxxxxxxxxxx --model claude-sonnet-4-20250514

# OpenAI
fword config set --provider openai --api-token sk-xxxxxxxxxxxx --model gpt-4o

# OpenRouter (access 100+ models with one key)
fword config set --provider openrouter --api-token sk-or-xxxxxxxxxxxx --model openai/gpt-5.4-mini

# Google Gemini
fword config set --provider gemini --api-token AIzaSy-xxxxxxxxxxxx --model gemini-1.5-flash

# Custom endpoint (Ollama, LiteLLM proxy, etc.)
fword config set --provider openai --api-token sk-xxxxxxxxxxxx --base-url http://localhost:11434/v1 --model llama3.2
```

Config is stored at `~/.config/fword/config.yaml`.

### 2. Add shell integration

```bash
# bash — add to ~/.bashrc
echo 'eval "$(/path/to/fword --shell-init bash)"' >> ~/.bashrc
source ~/.bashrc

# zsh — add to ~/.zshrc
echo 'eval "$(/path/to/fword --shell-init zsh)"' >> ~/.zshrc
source ~/.zshrc

# fish — add to ~/.config/fish/config.fish
echo '/path/to/fword --shell-init fish | source' >> ~/.config/fish/config.fish
```

### 3. Use it

Run any failing command, then type `fword`:

```bash
$ docker-compose up -d
ERROR: Version in "./docker-compose.yml" is unsupported.

$ fword
```

---

## Options

| Config key        | Default                     | Description                                           |
|-------------------|-----------------------------|-------------------------------------------------------|
| `provider`        | `claude`                    | AI backend: `claude`, `openai`, `openrouter`, `gemini`|
| `api_key`         | —                           | Authentication token for the provider                 |
| `model`           | `claude-sonnet-4-20250514`  | Model name                                            |
| `base_url`        | *(provider default)*        | Override API endpoint (proxies, local models)         |
| `max_tokens`      | `512`                       | Max tokens in AI response                             |

```bash
fword config show                       # view all settings
fword config set --provider openrouter --api-token sk-or-xxxx --base-url https://openrouter.ai/api/v1 --model openai/gpt-5.4-mini
fword --rerun                           # Re-run the command to get output for more accurate AI context (idempotent cmds only)
```

---

## Direct usage (no shell integration)

```bash
fword --cmd "kubectl get pods" --exit-code 1
fword --cmd "pnpm run build" --exit-code 1 --output "Cannot find module 'webpack'"
fword --cmd "docker compose up" --exit-code 1 --rerun
fword --cmd "pip install numpy" --exit-code 1
fword --cmd "cargo build" --exit-code 101 --debug    # show raw AI response
```

---

## `fword cat` — file-to-prompt formatter

Concatenate files or a directory into a prompt-ready string, with each file
wrapped in `<file path>…</file path>` tags.  Pipe the output directly into
your AI prompt or clipboard.

```bash
# Explicit files (like cat)
fword cat go.mod go.sum

# Walk a directory (respects .gitignore when inside a git repo)
fword cat ./internal
fword cat internal/

# Walk current directory (default when no args given)
fword cat
```

Example output:

```
<file go.mod>
module github.com/Rishang/fword
...
</file go.mod>
<file internal/config/config.go>
...
</file internal/config/config.go>
```

---

## How it works

1. Shell hook (`PROMPT_COMMAND` / `precmd`) records the last command and its exit code into env vars.
2. `fword` reads those vars, builds a prompt, and calls the configured AI provider.
3. AI returns either `FIX: <cmd>` or `STEPS:\n$ cmd1\n$ cmd2…`
4. `fword` prints corrected command(s) as plain text.

The AI prompt is intentionally terse: *commands only, no prose, no docs*.

---

## Building releases

```bash
task dist           # cross-compiles for linux/darwin/windows amd64+arm64
```
