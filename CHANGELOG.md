# Changelog

## v1.2.0 — 2026-08-02

### Added
- **Themes** — six built-in palettes (Catppuccin Mocha default, Macchiato, Frappé, Latte for light terminals, Dracula, Gruvbox), applied live from the new Theme menu and persisted to config.
- **Scenes** — twelve WiZ firmware scenes (Ocean, Romance, Sunset, Party, Fireplace, Cozy, Forest, Pastel, Wake-up, Bedtime, Daylight, Focus) with live preview while browsing and digit-key quick apply.
- **Device groups** — name a set of saved bulbs and target them together; power, brightness, color, temp, and scenes fan out concurrently with one aggregated result line.
- **Live dashboard** — 10-second sync heartbeat keeps state honest when the bulb changes from elsewhere; new Mode chip (WHITE + kelvin / COLOR + swatch) and live/stale sync indicator; commands that fail trigger an automatic quiet re-sync.
- **CLI mode** — `lumina on | off | color <hex> | temp <kelvin> | scene <name|id> | status | discover` run headless; bare `lumina` opens the TUI. Pairs with cron for scheduling.
- **Install scripts** — one-line installers for macOS/Linux (`install.sh`) and Windows (`install.ps1`).
- README screenshots generated from the real UI (`docs/screenshots/`).

### Fixed
- Installer now upgrades an existing `lumina` in place, preventing a stale copy earlier on PATH from shadowing the new version.
- Color Temp view rendered a blank panel (missing view case since the feature shipped).
- Group power toggle could get stuck one-way; now flips optimistically.
- Persisted theme now styles text inputs and spinner at boot.

## v1.1.1 — 2026-08-01

### Fixed
- `lumina -v` crashed (version args checked after flag parsing).
- UI froze on network commands — all device sends are now asynchronous with explicit success/error status levels.
- White-mode bulbs showed `#000000` as their color; color temp is now read, shown, and persisted.
- Boot could resolve the wrong bulb via an unrelated saved device's MAC.
- Hex input without `#` broke the color swatch; input is normalized.
- Config writes are atomic (temp file + rename).
- Status colors were keyword-guessed ("not found on network" showed as Success); levels are explicit now.
- `release.sh` declared `#!/bin/sh` while using bash features.

## v1.1.0 — earlier

Initial public feature set: color grid, hex input, brightness, color temp, sleep timer, discovery, saved devices, telemetry dashboard.
