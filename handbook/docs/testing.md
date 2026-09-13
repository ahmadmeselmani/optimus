---
sidebar_position: 4
---

# Manual Testing

Verify the current lightweight local-model interface against a running
Ollama instance.

## Setup

Use the Go toolchain specified in `go.mod`. Start Ollama separately with
`ollama serve` if needed, then run from the Optimus repository:

```bash
ollama pull qwen2.5:3b
ollama list
make build
make test
```

Expect an installed model, a binary at `bin/optimus`, and passing Go tests.

## Interactive chat

```bash
./bin/optimus
```

Expect the animated full mascot and wordmark, followed by the chat UI.
Press any key to skip the banner. Send `Hi`; expect a streamed model reply.
The header shows the active model, project, branch when available, and
cumulative token usage. Esc stops generation; ctrl+c quits.
Repeat sending and stopping replies; canceled streams should release their
response bodies even when the UI no longer reads their events.

Switch to another installed model through `/models` and send `Hi` again.
Expect the reply to start directly below the Optimus label, including when
the model streams leading blank lines. Paragraph breaks and code indentation
inside the reply should remain intact.

Run `/clear`; expect the conversation and counters to reset. The compact
mascot returns in the empty conversation. Run `/logo`; expect the compact
artwork with the same blue–chrome–red gradient.

## Commands and picker

Type `/` and use arrow keys to select a suggestion. Tab completes it;
enter completes and runs it. `/help` lists only the implemented commands.

Run `/models`; expect the models from `ollama list`. Arrow keys move the
selection, enter switches the active model, and esc cancels without switching.
Run `/model` to see connection details and `/context` to see token counts.
`/status` displays the directory, branch, and configured permissions.

Use `/model <installed-name>` to switch directly for the session.
With Ollama reachable, `/model definitely-not-a-real-model` should report
that the model is not installed and retain the active model.

## Persistent default model

```bash
./bin/optimus config model qwen2.5:3b
./bin/optimus config model
./bin/optimus config model qwen2.5:3b --project
```

Expect the saved config path and effective default for the current directory.
The global path is `~/.optimus/config.yaml`; `--project` writes to the current
directory's `.optimus/config.yaml`. Project settings override global settings.
These commands work without Ollama running. Other settings are preserved,
and model-only overrides inherit the provider and base URL.
Invalid existing config settings are rejected before saving, leaving the
file unchanged.

In the TUI, select a model through `/models` and run `/model default` to save
it globally. `/model default <installed-name> --project` saves and activates
a project default. Restart and check `/model` to verify persistence.
If a project overrides a saved global default, the TUI explains how to update
it. A failed save leaves the active model unchanged.

## One-off prompt

```bash
./bin/optimus ask "Reply with just OK"
```

Expect a streamed response and normal exit, without entering the TUI or
using repository tools.

## Existing repository assistant

From a repository:

```bash
./bin/optimus "List the top-level files in this repository"
```

Expect the existing read-only assistant to print tool activity when the model
requests it, then its answer. Available tools read directories, files, text
search results, Git status, and Git diffs. This path is separate from chat;
response quality depends on the local model's ability to follow its protocol.

## Connection errors

With Ollama stopped, send a prompt and expect a connection error. `/models`
should report that installed models could not be listed and show configured
model classes. Restart Ollama and try again.

## Documentation build

```bash
make handbook-build
```

Expect the static handbook to build without broken links.

## Release installation

Build all archives locally with `bash scripts/build-release.sh v0.1.0`.
Expect six archives, two installer scripts, and `checksums.txt` in `dist/`.
Verify the checksums with `(cd dist && sha256sum -c checksums.txt)` on Linux.

After publishing a public release, on each supported native platform run
the installer command from the overview. Expect a verified binary installed
on PATH. `optimus --version` should report the release tag, and `optimus`
should open the TUI from any directory without a Go installation. With
Ollama running, send a message and verify a reply. Repeat the installer to
check updating. On Windows, also open a new terminal and check PATH.

Before publication, reject corrupted archives and unsupported architectures
using installer fixtures; no binary should be installed on failed verification.
Cross-build success does not replace native platform checks.
