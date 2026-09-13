---
sidebar_position: 3
---

# Current Scope

Optimus is maintained as a lightweight terminal interface for local models.
The existing implementation is retained. No additional features or
coding-agent milestones are scheduled.

The focus is a polished TUI, reliable local Ollama connections, model
selection, and clear configuration.

## Available today

- Streaming interactive chat with cancellation and a scrolling conversation.
- Installed-model picker and direct session model switching.
- Persistent global and project default model settings.
- Slash-command suggestions, connection details, and token counters.
- Animated startup artwork and compact gradient artwork through `/logo`.
- One-off prompts through `optimus ask`.
- A separate read-only repository assistant through `optimus "<question>"`.

The interactive TUI streams chat directly through the model adapter. It
maintains conversation history in memory for the session. The repository
assistant is a separate CLI path and remains available as implemented.

## Distribution

Optimus is free and open source under the MIT license. Maintainers manually
trigger the release workflow with an existing version tag, review the draft,
and publish it. Users install a prebuilt binary and run `optimus` from their
terminal, with Ollama and local models installed separately.
