# Optimus — Maintenance Instructions

Optimus is a lightweight Go terminal interface for local Ollama models.
Keep the existing implementation and focus on a polished TUI, reliable local
connections, and easy model configuration. Do not implement additional
features unless explicitly requested by the user.

The current scope and behavior are documented in `README.md` and `handbook/docs/`.
The separate read-only repository assistant remains as implemented.

## Documentation

Keep the handbook aligned with shipped behavior. Document actual commands,
configuration, and controls without presenting unimplemented features as a
roadmap. Update architecture and manual checks when affected, and validate
the handbook with `make handbook-build` after documentation changes.

## Maintenance

Prefer small changes and existing dependencies. Preserve model interfaces,
configuration precedence, and the lightweight chat flow. Run appropriate Go
tests for behavior changes. Do not add frameworks, stub packages, panels,
agent infrastructure, or background services without a user request.
