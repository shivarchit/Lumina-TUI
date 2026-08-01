# Fix Round Design — Lumina-TUI

Date: 2026-08-01
Status: Approved (approach A: targeted fixes + thin async layer)
Scope: bug fixes only. Enhancements (scenes, groups, polling, CLI subcommands, goreleaser) deferred to a later round.

## Problems being fixed

1. **`lumina -v` crashes.** `flag.Parse()` runs before the version check in `internal/app/run.go`; `-v` is not a registered flag, so the flag package prints an error and exits(2).
2. **UI freezes on network commands.** Power toggle, brightness, color grid, hex input, and color temp all call `wiz.SendCommand` synchronously inside the Bubble Tea `Update` loop. With 3 retries + backoff this blocks the UI up to ~2s+ when the bulb is unreachable. Color grid preview sends on every arrow keypress.
3. **White-mode bulbs show black color.** `GetPilotState` builds `#000000` when the bulb is in color-temp mode (no r/g/b in the `getPilot` response); the state sync then overwrites `currentColor` with black.
4. **Wrong device resolved at boot.** `activeMac()` in `tui_state.go` falls back to `savedDevices[0].Mac` even when that device is unrelated to `cfg.IP`; MAC resolution then rewrites the target IP to a different bulb.
5. **Hex input without `#` breaks the swatch.** `HexToRGB` accepts `CBA6F7`, but `currentColor` is stored verbatim; lipgloss color needs the `#` prefix.
6. **`release.sh` shebang is `#!/bin/sh`** but the script uses bashisms (`BASH_SOURCE`, arrays, `[[`, `pipefail`). Breaks under strict POSIX sh.
7. **Status color detection is keyword sniffing.** `renderStatusBlock` matches substring "on", so "device not found on network" renders as Success.
8. **Minor bundle:** README says 24-color grid (palette has 21) and Go 1.18+ (go.mod requires 1.25); `config.Save` is a non-atomic write; color temp is never persisted; discovery dashboard bar hardcodes a fake `/10 bulbs` percentage.

## Design

### Async command layer (fixes 2, 7)

New message type in `internal/ui/tui_state.go`:

```go
type commandResultMsg struct {
    err        error
    elapsed    time.Duration
    onSuccess  func(*model)   // state mutation applied only on success
    successMsg string
    failPrefix string
}
```

New command builder:

```go
func sendCmd(ip, port, method string, params map[string]interface{},
    successMsg, failPrefix string, onSuccess func(*model)) tea.Cmd
```

`sendCmd` wraps `wiz.SendCommand` in a `tea.Cmd` goroutine and emits `commandResultMsg`. A single `Update` case handles it: record latency/telemetry via `recordCommand`, on error push error status with `failPrefix`, on success run `onSuccess(&m)` then push success status.

Converted callers: power toggle, `adjustBrightness`, color grid enter, color grid arrow preview, hex input apply, color temp send, in-process timer power-off. The pre-TUI auto power-on in `app.Run` stays synchronous.

Silent preview: the color grid arrow preview passes empty `successMsg` and `failPrefix`. The `commandResultMsg` handler treats both-empty as a silent send — no status push and no telemetry recording — so rapid arrow movement doesn't flood the status log or skew command-health stats.

Optimistic UI is dropped: state mutations (`isOn`, `brightness`, `currentColor`, `colorTemp`, `persistConfig`) move into `onSuccess`. No busy flag; commands are fast when the bulb is reachable, and duplicate keypresses just send more UDP packets, which is harmless.

Status levels replace keyword sniffing:

```go
type statusLevel int // statusInfo, statusSuccess, statusError

func pushStatus(m model, level statusLevel, s string) model
```

The model stores the current `statusLevel`; `renderStatusBlock` switches on it. The keyword `strings.Contains` block in `tui_view.go` is deleted.

### Surgical fixes

- **Version flag (1):** move the `os.Args` scan for `-v`/`--version`/`version` above `flag.Parse()` in `run.go`.
- **White-mode color (3):** in `GetPilotState`, if `r`, `g`, `b` are all absent, return `ColorHex: ""`. Also parse `temp` into a new `PilotState.Temp int`; the sync handler sets `m.colorTemp` when `Temp > 0`. The existing empty-string guard in `tui_update.go` already skips empty `ColorHex`.
- **Boot device resolution (4):** delete the `savedDevices[0]` fallback in `activeMac()`. Only an exact `saved.IP == cfg.IP` match returns a MAC; otherwise boot uses plain IP state sync.
- **Hex normalize (5):** on valid parse in `hexInputView`, store `currentColor = "#" + strings.ToUpper(strings.TrimPrefix(val, "#"))`.
- **Shebang (6):** `release.sh` line 1 becomes `#!/usr/bin/env bash`.
- **Minor bundle (8):**
  - README: "21-color grid", "Go 1.25+".
  - `config.Save`: write to a temp file in the same directory, then `os.Rename` over the target. Refactor to `SaveTo(path string, cfg Config)` with `Save(cfg)` wrapping it using `Path()`, so tests can target a temp dir.
  - Persist color temp: add `LastColorTemp int` to `Config`, restore in `NewModel`, include in `persistConfig`.
  - Discovery dashboard: drop the fake percentage bar; show "found N in Xms" plus the existing latency sparkline.

### Error handling

All send errors flow through the single `commandResultMsg` case and render as error-level status. Discovery and state-sync paths are already async and keep their existing handling, updated only to pass explicit status levels.

### Testing

- `GetPilotState` white-mode: UDP responder returns `{"result":{"state":true,"dimming":50,"temp":4000}}`; assert `ColorHex == ""` and `Temp == 4000`.
- `activeMac`: table test — exact IP match, no match, unrelated first saved device (must return "").
- Hex normalization: drive `Update` with key messages in `hexInputView`, assert stored color has `#` prefix and uppercase.
- Config atomic save: `SaveTo` + `Load` roundtrip against a temp dir path.
- Version flag: manual verification (`go run ./internal -v`) since it exits the process.
- All existing tests stay green: `go test ./...`.

## Out of scope

Scenes, multi-bulb groups, periodic polling, CLI subcommands, goreleaser migration, command queue/debounce machinery.
