---
sidebar_position: 2
---

# Architecture

Optimus is a Go CLI with a Bubble Tea and Lip Gloss TUI. The interactive
session streams chat directly through the Ollama model adapter, retaining
conversation history in memory. It does not run the repository agent loop.
Leading blank lines in assistant replies are removed only for display, during
streaming and after completion; the stored model response is preserved.

## Entrypoints

| Invocation | Behavior |
| --- | --- |
| `optimus` | Load configuration and start interactive chat |
| `optimus ask "<prompt>"` | Stream a one-off model response without tools |
| `optimus config model [<name>] [--project]` | Show or save the default model without contacting Ollama |
| `optimus "<repository question>"` | Run the existing bounded, read-only repository assistant |
| `optimus help` | Show usage |

All generation paths use `models.fast`. Configuration merges built-in
values, global settings, and project settings, with project settings taking
precedence. Model fields merge individually. Saving a default updates only
`models.fast.model`, preserves other settings, and replaces the config file
atomically.

## Packages

| Package | Responsibility |
| --- | --- |
| `cmd/optimus` | CLI dispatch, model construction, and session startup |
| `internal/config` | Defaults, layered YAML loading, persistent model settings |
| `internal/model` | Shared model interface and Ollama adapter; chat streams use `/api/chat`, installed models use `/api/tags` |
| `internal/tui` | Streaming conversation, cancellation, header, token counters, input, slash commands, and model picker |
| `internal/logging` | Configurable structured logging |
| `internal/repository` | File scanning and text search for the separate repository assistant |
| `internal/tools` | Read-only directory, file, search, Git status, and Git diff tools |
| `internal/agent` | Bounded tool loop for the separate repository assistant |

The TUI depends on model interfaces, using optional `ModelLister` and
`ModelSwitcher` capabilities for the picker and session switching. Ollama
is the currently supported provider.

## Artwork

Embedded assets live in `internal/tui/assets/`. The startup banner uses
`optimus_full.txt`, scaled to fit through the braille bitmap scaler, followed
by an animated OPTIMUS wordmark using the `speed` font. The empty conversation
and `/logo` use `optimus_compact.txt`. Artwork shares a blue–chrome–red
gradient. Any key skips the startup animation.

## Token display

Session and last-turn counts come from the model responses. The header's
Context numerator is cumulative session usage, not a measurement of the
current history's token size. `/clear` resets the history and counters.

## Release distribution

A maintainer manually triggers `.github/workflows/release.yml` with an
existing version tag. Pushing a tag does not run it. It tests and vets the
code, invokes `scripts/build-release.sh` to package six CGO-free binaries
(Linux/macOS/Windows, amd64/arm64), embeds the version, and creates a draft
GitHub Release. Archives include the README and license. Checksums and Unix
and PowerShell installers are also uploaded. Public installation requires
a public repository and a published release. This adds no application service
or runtime dependency; Ollama and models remain separate.
