# Architecture

Lumina-TUI follows a package-oriented Go layout.

## Top-level layout

- `internal/main.go` — executable entrypoint.
- `internal/app` — startup flow and CLI mode handling.
- `internal/config` — config validation and persistence.
- `internal/ui` — Bubble Tea model, update loop, and rendering.
- `internal/wiz` — WiZ UDP networking and device discovery.
- `internal/version` — application version constant.
- `build/release.sh` — cross-platform release script.
- `docs/` — project documentation.

## Runtime flow

1. `internal/main.go` calls `app.Run()`.
2. `app` loads config and initializes the TUI model from `ui`.
3. `ui` handles user interaction and delegates network operations to `wiz`.
4. `wiz` sends UDP commands and performs discovery.
