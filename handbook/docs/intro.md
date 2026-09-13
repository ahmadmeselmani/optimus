---
sidebar_position: 1
slug: /
---

# Optimus

Optimus is a lightweight terminal interface for local Ollama models, written
in Go. It provides streaming chat, an installed-model picker, persistent
default model settings, and a terminal UI built with Bubble Tea and Lip Gloss.

Optimus is free and open source, with no account or payment required.

## Install a release

After the first public release is published:

```bash
curl -fsSL https://github.com/ahmadmeselmani/optimus/releases/latest/download/install.sh | sh
optimus
```

On Windows, use PowerShell:

```powershell
irm https://github.com/ahmadmeselmani/optimus/releases/latest/download/install.ps1 | iex
optimus
```

Install Ollama and a model separately. Prebuilt binaries need no Go toolchain.
The installers verify SHA-256 checksums and install the command on PATH.
Run `optimus --version` to see the installed release version.

## Quick start from source

Start Ollama with `ollama serve` if it is not already running. In another
terminal, from the Optimus repository:

```bash
ollama pull qwen2.5:3b
make build
./bin/optimus
```

Type a message and press enter. `/models` opens the installed-model picker;
use arrow keys and enter to select, or esc to cancel. `/model default`
saves your current model for future launches.

## Controls

| Key | Action |
| --- | --- |
| Enter | Send a message |
| Esc | Stop generation or cancel the model picker |
| Ctrl+C | Quit |
| ↑ / ↓ | Select a model or slash-command suggestion |
| Tab | Complete a slash-command suggestion |

Type `/help` to see commands. Typing `/` opens command suggestions.
Any key skips the animated startup banner.

## Slash commands

| Command | Action |
| --- | --- |
| `/help` | List commands |
| `/models` | Pick an installed Ollama model |
| `/model` | Show the active model and connection details |
| `/model <name>` | Switch the model for this session |
| `/model default [<name>] [--project]` | Save a default and activate it; omitted name saves the current model |
| `/logo` | Show the compact Optimus art with the blue–chrome–red gradient |
| `/status` | Show project, directory, branch, and configured permissions |
| `/context` | Show session and last-turn token counts, plus the model's context window |
| `/clear` | Clear the conversation, history, and token counters |
| `/bye`, `/quit`, `/exit` | Quit |

## Configuration

```bash
./bin/optimus config model qwen2.5:3b
./bin/optimus config model
./bin/optimus config model qwen2.5:3b --project
```

Settings merge in this order: built-in defaults, `~/.optimus/config.yaml`,
then the current directory's `.optimus/config.yaml`. Project settings take
precedence. Saving a model preserves other settings.

To change the Ollama connection, edit either config file:

```yaml
models:
  fast:
    provider: ollama
    model: qwen2.5:3b
    base_url: http://localhost:11434
```

The `fast` entry is the active model used by all entrypoints. Individual
model fields inherit from earlier configuration layers.

## Project scope

Keep Optimus focused on local model connectivity and a polished, lightweight
TUI. See [Current Scope](/milestones), [Architecture](/architecture),
and [Manual Testing](/testing).
