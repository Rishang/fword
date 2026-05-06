# fk 🐷

> **You typed it wrong. `fk` fixes it.**

<p align="center">
  <img src="https://img.shields.io/github/v/release/Rishang/fk?style=flat-square&color=ff69b4" alt="release">
  <img src="https://img.shields.io/github/stars/Rishang/fk?style=flat-square&color=yellow" alt="stars">
  <img src="https://img.shields.io/github/license/Rishang/fk?style=flat-square" alt="license">
  <img src="https://img.shields.io/badge/built%20with-Go-00ADD8?style=flat-square&logo=go" alt="go">
  <img src="https://img.shields.io/badge/zero-dependencies-brightgreen?style=flat-square" alt="zero deps">
</p>

<p align="center">
  AI-powered shell command corrector.<br>
  Run a broken command, type <code>fk</code> — get the fix instantly.<br>
  Built in Go. Zero runtime dependencies. Works with any AI provider.
</p>

```
$ git psuh origin main
fatal: 'psuh' is not a git command. See 'git --help'.

$ fk
  git push origin main
  ✔ run it? [y/n]
```

```
$ docker-compose up -d
ERROR: Version in "./docker-compose.yml" is unsupported.

$ fk
  docker compose up -d

```

You can also use shell comment to provide prompt as well:
```
$ # sync OS system time which currently is wrong
$ curl https://example.com
curl: (60) SSL certificate problem: certificate is not yet valid

$ fk

sudo timedatectl set-ntp true

```

Inspired by [thefuck](https://github.com/nvbn/thefuck). Faster. No Python. No rules to maintain.

---

## Table of Contents

- [Install](#install)
- [Setup](#setup)
- [How it works](#how-it-works)
- [Flags](#flags)
- [Config](#config)
- [`fk cat`](#fk-cat--files-to-prompt)
- [Building](#building)

---

## Install

### With [install-release](https://pypi.org/project/install-release/) (`ir`) — recommended

```bash
pip install -U install-release
ir get https://github.com/Rishang/fk
```

Binaries go to `~/bin` — make sure it's on your PATH:

```bash
export PATH="$HOME/bin:$PATH"
```

### Pre-built binary

Grab the binary for your platform from the [latest release](https://github.com/Rishang/fk/releases/latest):

| Platform    | Asset              |
|-------------|--------------------|
| Linux x64   | `fk-linux-amd64`  |
| Linux arm64 | `fk-linux-arm64`  |
| macOS x64   | `fk-darwin-amd64` |
| macOS arm64 | `fk-darwin-arm64` |

```bash
# Example: Linux x86_64
curl -fL -o fk https://github.com/Rishang/fk/releases/latest/download/fk-linux-amd64
chmod +x fk
sudo mv fk /usr/local/bin/
```

### From source

```bash
git clone https://github.com/Rishang/fk
cd fk
task install        # installs to $(go env GOPATH)/bin/fk
```

> Requires Go 1.21+

---

## Setup

### 1. Pick your AI provider

`fk` works with any major AI provider — or your own local model:

```bash
# Claude (Anthropic)
fk config set --provider claude --api-token sk-ant-xxxx --model claude-sonnet-4-20250514

# OpenAI
fk config set --provider openai --api-token sk-xxxx --model gpt-4o

# OpenRouter (100+ models, one key)
fk config set --provider openrouter --api-token sk-or-xxxx --model openai/gpt-4o-mini

# Google Gemini
fk config set --provider gemini --api-token AIzaSy-xxxx --model gemini-1.5-flash

# Local model (Ollama, LiteLLM, etc.)
fk config set --provider openai --api-token x --base-url http://localhost:11434/v1 --model llama3.2
```

Config lives at `~/.config/fk/config.yaml`.

### 2. Hook into your shell

```bash
# bash — add to ~/.bashrc
echo 'eval "$(fk --shell-init bash)"' >> ~/.bashrc && source ~/.bashrc

# zsh — add to ~/.zshrc
echo 'eval "$(fk --shell-init zsh)"' >> ~/.zshrc && source ~/.zshrc

# fish — add to ~/.config/fish/config.fish
echo 'fk --shell-init fish | source' >> ~/.config/fish/config.fish
```

### 3. Break things. Fix them.

```bash
$ kubectll get pods
command not found: kubectll

$ fk
  kubectl get pods
  ✔ run it? [y/n]
```

---

## How it works

```
  failing command + exit code
           │
           ▼
    shell hook captures context
    (PROMPT_COMMAND / precmd)
           │
           ▼
       fk builds a prompt
           │
           ▼
      AI returns the fix
    (commands only, no prose)
           │
           ▼
       fk prints the fix
```

1. The shell hook captures the last **non-fk** command and its exit code into env vars before each prompt.
2. `fk` builds a terse prompt and calls your configured AI provider.
3. The AI responds with `FIX: <cmd>` or `STEPS:\n$ cmd1\n$ cmd2…`
4. `fk` presents the fix. Optionally runs it for you with `--auto-run`.

---

## Flags

| Flag                | Description                                                       |
|---------------------|-------------------------------------------------------------------|
| `--rerun` / `-r`    | Re-run the failed command to capture live output before asking AI |
| `--auto-run`        | Run the fix immediately without prompting                         |
| `--verbose` / `-v`  | Print prompts sent to the LLM and estimated token counts          |
| `--debug`           | Print raw AI response before parsing                              |
| `--cmd`             | Provide the failed command explicitly (no shell hook needed)      |
| `--exit-code`       | Provide the exit code explicitly                                  |
| `--output`          | Provide captured output explicitly                                |
| `--shell`           | Override shell detection (`bash`\|`zsh`\|`fish`)                 |
| `--version`         | Print the version                                                 |

```bash
# Direct usage — no shell integration needed
fk --cmd "kubectl get pods" --exit-code 1
fk --cmd "cargo build" --exit-code 101 --rerun --debug
fk --cmd "pip install numpy" --exit-code 1 --auto-run

# Debug: see exactly what's sent to the LLM
fk -v
```

---

## Config

| Key          | Default                    | Description                                            |
|--------------|----------------------------|--------------------------------------------------------|
| `provider`   | `claude`                   | AI backend: `claude`, `openai`, `openrouter`, `gemini` |
| `api_key`    | —                          | Your provider API token                                |
| `model`      | `claude-sonnet-4-20250514` | Model name                                             |
| `base_url`   | *(provider default)*       | Override endpoint — for proxies or local models        |
| `max_tokens` | `512`                      | Max tokens in AI response                              |
| `auto_run`   | `false`                    | Execute suggestion without confirmation                |

```bash
fk config show          # view current settings
fk config set --provider openai --api-token sk-xxxx --model gpt-4o
```

---

## `fk cat` — files to prompt

`fk cat` is a separate helper from the command corrector. Its main use case is **bringing local file context into a browser chat** — paste the output into the [ChatGPT](https://chatgpt.com) or [Claude](https://claude.ai) console (or any similar UI) so the model sees your repo without uploading files one by one. It bundles paths into one prompt-friendly blob with clear `<file …>` boundaries. It does **not** call the AI provider you configured for `fk`; it only reads the filesystem and prints to stdout.

**Behavior:**

- **Directories:** In a git repo, files are listed with `git ls-files` so tracked and untracked-but-not-ignored paths match `.gitignore`. Outside git, it walks the tree and skips hidden files and directories.
- **Files:** Only UTF-8 text is included; binary and permission-denied files are skipped. Paths in the output are relative to the current working directory.

```bash
fk cat go.mod go.sum          # specific files
fk cat ./internal             # walk a directory (respects .gitignore in repos)
fk cat                        # walk current directory (.)
fk cat pkg/ > /tmp/ctx.txt    # paste into ChatGPT / Claude web, etc.
```

Output:

```
<file go.mod>
module github.com/Rishang/fk
...
</file go.mod>
<file internal/config/config.go>
...
</file internal/config/config.go>
```

---

## Building

```bash
task build        # build for current platform → ./dist/fk
task dist         # cross-compile: linux/darwin/windows amd64+arm64
task test         # run tests
task install      # install to $GOPATH/bin
```

---

<p align="center">
  If <code>fk</code> saved you time, consider giving it a ⭐
</p>
