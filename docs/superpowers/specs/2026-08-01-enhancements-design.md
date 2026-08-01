# Enhancement Round Design — Lumina-TUI

Date: 2026-08-01
Status: Approved (user: "implement" after mockup review)
Design source: published mockup artifact "Lumina-TUI — Enhancement Mockups" (five screens + rejected list). This doc condenses the decisions.

## Features (build order: smallest risk first)

### 1. Themes
Named palette map replacing the hardcoded Catppuccin Mocha package vars. Six themes: Mocha (default), Macchiato, Frappé, Latte (light terminals), Dracula, Gruvbox. Config gains `"theme":"mocha"`; unknown/absent → Mocha. New Theme view in the menu; Enter applies immediately (package color vars reassigned; Bubble Tea re-renders every frame, so switching is live) and persists. No hot-reload machinery.

### 2. Scenes
WiZ built-in scenes via `setPilot {"sceneId": N}` through the existing async `sendCmd`. New Scenes view mirroring the color grid: 3-column grid, arrow navigation with silent live preview, Enter applies with status, digits 1-9/0 quick-apply the first ten. Twelve curated scenes (real WiZ IDs): Ocean 1, Romance 2, Sunset 3, Party 4, Fireplace 5, Cozy 6, Forest 7, Pastel 8, Wake-up 9, Bedtime 10, Daylight 12, Focus 15. `lastScene` persisted (int, 0 = none). Speed slider from the mockup is cut (YAGNI — firmware default speed; add when asked).

### 3. Live polling + mode awareness
- `tea.Tick` every 10s while in menuView fires a **quiet** state sync (no "State synced" status spam; telemetry still recorded).
- Model tracks `lastSyncAt time.Time` and `whiteMode bool` (Temp>0 and no ColorHex in last sync → white). Core dashboard panel shows `Mode WHITE 4000K` or `Mode COLOR <swatch> #hex`, and a `Sync live · Ns ago` line (red "stale" when last sync errored or >30s old).
- Re-sync on command failure: the `commandResultMsg` error branch chains a quiet sync — reconciles the three deferred minors from the fix round (optimistic-step drift, toggle double-press, stragglers).

### 4. CLI subcommands
Verb dispatch in `internal/app` before the TUI starts: `lumina on | off | color <hex> | temp <kelvin> | scene <name|id> | status | discover`. Bare `lumina` opens the TUI unchanged. Shares `loadRuntimeConfig` and the wiz client. `status` prints one line per saved device (falls back to the active target when none saved). `--group` flag and `off --after` sugar deferred to post-groups; existing `--timer` flags unchanged.

### 5. Groups
Config gains `groups: [{name, macs[]}]`. New Groups view: list groups with member count and expand; `n` creates a group (name prompt), Space toggles the highlighted saved device's membership while a group is selected, `d` deletes a group, Enter targets the group. While a group is targeted, device commands fan out via a new `fanoutCmd(ips, port, method, params, label)` — concurrent sends, one aggregated result message ("Living Room → brightness 72% (3/3 ok)"; failures listed by name). Selecting a single device clears the group target.

## Out of scope (rejected list from mockup)
Circadian scheduling (cron + CLI covers it), fades/ramps, mouse support, custom user scenes, keybinding config, theme hot-reload watcher.

## Error handling
Everything routes through existing statusLevel + commandResultMsg/fanout aggregate. Quiet syncs record telemetry but never push status on success; a failed quiet sync flips the Sync line to stale rather than spamming errors.

## Testing
Per feature: theme map lookup/fallback; scene payload + digit mapping; quiet-sync flag behavior and whiteMode derivation; CLI verb dispatch (unit-level arg parsing); group membership toggle + fanout aggregation (loopback UDP servers). Existing suite stays green.
