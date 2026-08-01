# Enhancement Round Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the five approved enhancements: Themes, Scenes, live polling + mode awareness, CLI subcommands, Groups (spec: `docs/superpowers/specs/2026-08-01-enhancements-design.md`).

**Architecture:** Every feature composes from the shipped machinery — async `sendCmd`/`commandResultMsg`, `pushStatus` levels, MAC-keyed saved devices, persisted config. Themes reassign the eight package color vars from a map. Scenes and Groups are new `sessionState` views mirroring existing view patterns. Polling is a `tea.Tick` loop feeding the existing sync command with a new quiet flag. CLI is a verb switch in `internal/app` before `tea.NewProgram`.

**Tech Stack:** Go 1.25, Bubble Tea v1.3.10, lipgloss v1.1.0. Module `wiz-tui`, main package at `./internal`.

## Global Constraints

- Existing tests must stay green after every task: `go test ./...` from repo root.
- Test placement: black-box tests in `tests/<pkg>/` (external package); white-box tests for unexported identifiers in-package as `internal/ui/*_internal_test.go` (files `tui_state_internal_test.go`, `tui_update_internal_test.go`, `tui_view_internal_test.go` already exist — append, don't recreate).
- Commit after each task. Plain commit messages — no co-author trailers.
- Menu order after this round (indices matter for the digit-jump handler): `Toggle Power, Scenes, Groups, Color Grid, Hex Colors, Brightness, Color Temp, Sleep Timer, Discover Devices, Saved Devices, Theme, Help, Exit` with icons `PWR, SCN, GRP, CLR, HEX, BRT, CCT, TMR, DSC, SAV, THM, HLP, EXT`. Digit keys map 1-9 then 0 to the first ten entries; Theme/Help/Exit are reachable by cursor only.
- Config JSON stays backward compatible: all new fields `omitempty`, absent values fall back to current behavior.

---

### Task 1: Themes

**Files:**
- Modify: `internal/config/config.go` (Config struct)
- Modify: `internal/ui/tui_state.go` (theme map, applyTheme, model field, NewModel, persistConfig, menu entries)
- Modify: `internal/ui/tui_update.go` (menu case indices, new themesView case)
- Modify: `internal/ui/tui_view.go` (themesView panel)
- Test: append to `internal/ui/tui_state_internal_test.go`

**Interfaces:**
- Consumes: existing `pushStatus(m, level, s)`, `persistConfig`.
- Produces: `type theme struct { mauve, blue, green, red, text, subtext, surface, base string }`; `var themes = map[string]theme{...}`; `var themeOrder = []string{...}`; `func applyTheme(name string) string` (returns the resolved name, falling back to "mocha"); `Config.Theme string`; model fields `themeName string`, `themeCursor int`; new `themesView sessionState`. Task 2 renumbers nothing here — this task establishes the final menu order from Global Constraints (Scenes/Groups entries are added now as placeholder rows that show "coming in this round" status when selected; Tasks 2 and 5 wire them).

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui_state_internal_test.go`:

```go
func TestApplyThemeFallsBackToMocha(t *testing.T) {
	if got := applyTheme("nonsense"); got != "mocha" {
		t.Fatalf("unknown theme must resolve to mocha, got %q", got)
	}
	if got := applyTheme(""); got != "mocha" {
		t.Fatalf("empty theme must resolve to mocha, got %q", got)
	}
	if got := applyTheme("latte"); got != "latte" {
		t.Fatalf("known theme must resolve to itself, got %q", got)
	}
	if string(mauve) != themes["latte"].mauve {
		t.Fatalf("applyTheme must reassign package colors: mauve=%s want %s", mauve, themes["latte"].mauve)
	}
	applyTheme("mocha") // restore for other tests
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestApplyTheme -v`
Expected: FAIL — compile error `undefined: applyTheme` / `themes`.

- [ ] **Step 3: Implement the theme map**

In `internal/ui/tui_state.go`, replace the color `var (...)` block (keep the same variable names — every render call site keeps working):

```go
type theme struct {
	mauve, blue, green, red, text, subtext, surface, base string
}

var themes = map[string]theme{
	"mocha":     {"#CBA6F7", "#89B4FA", "#A6E3A1", "#F38BA8", "#CDD6F4", "#6C7086", "#313244", "#1E1E2E"},
	"macchiato": {"#C6A0F6", "#8AADF4", "#A6DA95", "#ED8796", "#CAD3F5", "#6E738D", "#363A4F", "#24273A"},
	"frappe":    {"#CA9EE6", "#8CAAEE", "#A6D189", "#E78284", "#C6D0F5", "#737994", "#414559", "#303446"},
	"latte":     {"#8839EF", "#1E66F5", "#40A02B", "#D20F39", "#4C4F69", "#9CA0B0", "#CCD0DA", "#EFF1F5"},
	"dracula":   {"#BD93F9", "#8BE9FD", "#50FA7B", "#FF5555", "#F8F8F2", "#6272A4", "#44475A", "#282A36"},
	"gruvbox":   {"#D3869B", "#83A598", "#B8BB26", "#FB4934", "#EBDBB2", "#928374", "#3C3836", "#282828"},
}

// themeOrder fixes menu ordering (maps are unordered).
var themeOrder = []string{"mocha", "macchiato", "frappe", "latte", "dracula", "gruvbox"}

var (
	mauve   lipgloss.Color
	blue    lipgloss.Color
	green   lipgloss.Color
	red     lipgloss.Color
	textCol lipgloss.Color
	subtext lipgloss.Color
	surface lipgloss.Color
	base    lipgloss.Color
)

// applyTheme reassigns the package color vars; unknown names fall back to mocha.
// ponytail: package-global palette, fine for a single-model TUI process.
func applyTheme(name string) string {
	t, ok := themes[name]
	if !ok {
		name = "mocha"
		t = themes[name]
	}
	mauve, blue, green, red = lipgloss.Color(t.mauve), lipgloss.Color(t.blue), lipgloss.Color(t.green), lipgloss.Color(t.red)
	textCol, subtext, surface, base = lipgloss.Color(t.text), lipgloss.Color(t.subtext), lipgloss.Color(t.surface), lipgloss.Color(t.base)
	return name
}

func init() { applyTheme("mocha") }
```

- [ ] **Step 4: Config + model plumbing**

`internal/config/config.go` — add to Config:

```go
	Theme          string        `json:"theme,omitempty"`
```

`internal/ui/tui_state.go`:
- Add `themesView` to the sessionState const block (append after `helpView`).
- Model struct: add `themeName string` and `themeCursor int`.
- `NewModel`: before the return, `themeName := applyTheme(cfg.Theme)`; set `themeName: themeName` in the literal. Set `themeCursor` to the index of themeName in themeOrder (small loop).
- Replace `choices`/`icons` in the literal with the Global Constraints menu order:

```go
		choices: []string{"Toggle Power", "Scenes", "Groups", "Color Grid", "Hex Colors", "Brightness", "Color Temp", "Sleep Timer", "Discover Devices", "Saved Devices", "Theme", "Help", "Exit"},
		icons:   []string{"PWR", "SCN", "GRP", "CLR", "HEX", "BRT", "CCT", "TMR", "DSC", "SAV", "THM", "HLP", "EXT"},
```

- `persistConfig`: add `Theme: m.themeName,`.

- [ ] **Step 5: Update loop**

`internal/ui/tui_update.go` menuView enter switch — renumber to the new order:

```go
				case 0: // Toggle Power  (unchanged body)
				case 1: // Scenes
					m = pushStatus(m, statusInfo, "Scenes arrive later this round")
				case 2: // Groups
					m = pushStatus(m, statusInfo, "Groups arrive later this round")
				case 3: // Color Grid       -> colorPickerView (old case 1 body)
				case 4: // Hex Colors       -> old case 2 body
				case 5: // Brightness       -> old case 3 body
				case 6: // Color Temp       -> old case 4 body
				case 7: // Sleep Timer      -> old case 5 body
				case 8: // Discover Devices -> old case 6 body
				case 9: // Saved Devices    -> old case 7 body
				case 10: // Theme
					m.state = themesView
				case 11: // Help             -> helpView
				case 12: // Exit             -> tea.Quit
```

(Bodies move verbatim; only indices change.)

Add the themesView case to the state switch:

```go
			case themesView:
				switch msg.String() {
				case "esc", "q":
					m.state = menuView
				case "up", "k":
					if m.themeCursor > 0 {
						m.themeCursor--
					}
				case "down", "j":
					if m.themeCursor < len(themeOrder)-1 {
						m.themeCursor++
					}
				case "enter":
					m.themeName = applyTheme(themeOrder[m.themeCursor])
					m.persistConfig()
					m = pushStatus(m, statusSuccess, "Theme: "+m.themeName)
					m.state = menuView
				}
```

- [ ] **Step 6: View**

`internal/ui/tui_view.go` — add to the leftPanel state switch:

```go
	case themesView:
		leftPanel = sectionHeader("Theme", "Enter applies · persists") + "\n\n"
		for i, name := range themeOrder {
			t := themes[name]
			row := "  " + name
			style := lipgloss.NewStyle().Foreground(subtext)
			if i == m.themeCursor {
				row = "> " + name
				style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
			}
			swatches := ""
			for _, c := range []string{t.mauve, t.blue, t.green, t.red, t.text} {
				swatches += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("██")
			}
			active := ""
			if name == m.themeName {
				active = lipgloss.NewStyle().Foreground(green).Render("  active")
			}
			leftPanel += fmt.Sprintf("%-14s %s%s\n", style.Render(row), swatches, active)
		}
```

- [ ] **Step 7: Run tests, commit**

Run: `go test ./...` — all green (the existing black-box menu test asserts "Control Board", unaffected).

```bash
git add internal/ tests/
git commit -m "feat: theme support - six named palettes, persisted, live-applied"
```

---

### Task 2: Scenes

**Files:**
- Modify: `internal/config/config.go` (LastScene field)
- Modify: `internal/ui/tui_state.go` (scene list, scenesView state, model field, persistConfig)
- Modify: `internal/ui/tui_update.go` (menu case 1 wiring, scenesView case)
- Modify: `internal/ui/tui_view.go` (scenes grid panel)
- Test: append to `internal/ui/tui_state_internal_test.go`

**Interfaces:**
- Consumes: `sendCmd` (silent contract for previews), `pushStatus`, `persistConfig`, menu order from Task 1.
- Produces: `var scenes = []struct{ name string; id int }{...}` (12 entries); `scenesView sessionState`; model fields `sceneCursor int`, `lastScene int`; `Config.LastScene int`; `func sceneParams(id int) map[string]interface{}` returning `{"sceneId": id}`.

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui_state_internal_test.go`:

```go
func TestSceneParams(t *testing.T) {
	p := sceneParams(4)
	if p["sceneId"] != 4 {
		t.Fatalf("sceneParams(4) = %v", p)
	}
	if len(scenes) != 12 {
		t.Fatalf("expected 12 curated scenes, got %d", len(scenes))
	}
	if scenes[0].name != "Ocean" || scenes[0].id != 1 {
		t.Fatalf("first scene must be Ocean/1, got %+v", scenes[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestSceneParams -v`
Expected: FAIL — compile error `undefined: sceneParams`.

- [ ] **Step 3: Implement**

`internal/ui/tui_state.go`:

```go
// scenes are WiZ firmware built-ins addressed by sceneId.
var scenes = []struct {
	name string
	id   int
}{
	{"Ocean", 1}, {"Romance", 2}, {"Sunset", 3},
	{"Party", 4}, {"Fireplace", 5}, {"Cozy", 6},
	{"Forest", 7}, {"Pastel", 8}, {"Wake-up", 9},
	{"Bedtime", 10}, {"Daylight", 12}, {"Focus", 15},
}

func sceneParams(id int) map[string]interface{} {
	return map[string]interface{}{"sceneId": id}
}
```

- Add `scenesView` to the sessionState consts; model fields `sceneCursor int`, `lastScene int`.
- `NewModel`: `lastScene: cfg.LastScene,`.
- `persistConfig`: `LastScene: m.lastScene,`.
- `internal/config/config.go`: `LastScene int \`json:"lastScene,omitempty"\`` in Config.

`internal/ui/tui_update.go` — menu case 1 becomes `m.state = scenesView`. Add:

```go
			case scenesView:
				preview := func() {
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
						sceneParams(scenes[m.sceneCursor].id), "", "", nil)) // silent preview
				}
				apply := func(idx int) {
					s := scenes[idx]
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot", sceneParams(s.id),
						"Scene: "+s.name, "Scene failed",
						func(mm *model) {
							mm.lastScene = s.id
							mm.isOn = true
							mm.persistConfig()
						}))
					m.state = menuView
				}
				switch msg.String() {
				case "esc", "q":
					m.state = menuView
				case "up", "k":
					if m.sceneCursor >= 3 {
						m.sceneCursor -= 3
						preview()
					}
				case "down", "j":
					if m.sceneCursor < len(scenes)-3 {
						m.sceneCursor += 3
						preview()
					}
				case "left", "h":
					if m.sceneCursor > 0 {
						m.sceneCursor--
						preview()
					}
				case "right", "l":
					if m.sceneCursor < len(scenes)-1 {
						m.sceneCursor++
						preview()
					}
				case "enter":
					apply(m.sceneCursor)
				case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
					idx, _ := strconv.Atoi(msg.String())
					if idx == 0 {
						idx = 10
					}
					idx--
					if idx < len(scenes) {
						m.sceneCursor = idx
						apply(idx)
					}
				}
```

`internal/ui/tui_view.go` — scenes grid (mirror colorPickerView layout):

```go
	case scenesView:
		leftPanel = sectionHeader("Scenes", "WiZ built-ins · Enter applies · digits quick-apply") + "\n\n"
		for i, s := range scenes {
			label := fmt.Sprintf("%-10s", s.name)
			style := lipgloss.NewStyle().Foreground(textCol)
			prefix := "  "
			if i == m.sceneCursor {
				style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
				prefix = "> "
			}
			marker := " "
			if s.id == m.lastScene {
				marker = lipgloss.NewStyle().Foreground(green).Render("*")
			}
			leftPanel += style.Render(prefix+label) + marker + " "
			if (i+1)%3 == 0 {
				leftPanel += "\n"
			}
		}
```

- [ ] **Step 4: Run tests, commit**

Run: `go test ./...` — green.

```bash
git add internal/
git commit -m "feat: scenes view - twelve WiZ built-ins with preview and quick-apply"
```

---

### Task 3: Live polling + mode awareness + failure re-sync

**Files:**
- Modify: `internal/ui/tui_state.go` (quiet flag on sync msg/cmd, model fields, pollTick)
- Modify: `internal/ui/tui_update.go` (tick loop, quiet handling, whiteMode, failure re-sync)
- Modify: `internal/ui/tui_state.go` `renderDashboard` (Mode + Sync lines)
- Test: append to `internal/ui/tui_update_internal_test.go`

**Interfaces:**
- Consumes: `syncDeviceStateCmd`, `stateSyncResultMsg`, `commandResultMsg` handler.
- Produces: `stateSyncResultMsg` gains field `quiet bool`; `syncDeviceStateCmd(ip, port string, quiet bool) tea.Cmd` (signature change — update ALL existing call sites with `quiet=false`: two in `Init`, one in `macResolutionResultMsg`, one in discovery enter, two in savedDevices enter); `type pollTickMsg struct{}`; `func pollCmd() tea.Cmd` (10s tea.Tick); model fields `lastSyncAt time.Time`, `lastSyncOK bool`, `whiteMode bool`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/tui_update_internal_test.go`:

```go
func TestQuietSyncSkipsStatusAndSetsMode(t *testing.T) {
	m := testModel()
	before := m.status
	msg := stateSyncResultMsg{
		state: wiz.PilotState{Power: true, Brightness: 50, Temp: 4000},
		quiet: true, elapsed: 5 * time.Millisecond,
	}
	updated, _ := m.Update(msg)
	um := updated.(model)
	if um.status != before {
		t.Fatalf("quiet sync must not push status, got %q", um.status)
	}
	if !um.whiteMode {
		t.Fatal("Temp>0 with no color must set whiteMode")
	}
	if !um.lastSyncOK || um.lastSyncAt.IsZero() {
		t.Fatal("sync bookkeeping not recorded")
	}
}

func TestCommandFailureTriggersResync(t *testing.T) {
	m := testModel()
	msg := commandResultMsg{err: errors.New("timeout"), elapsed: time.Millisecond,
		successMsg: "x", failPrefix: "y"}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("command failure must chain a re-sync command")
	}
}
```

(Add `"wiz-tui/internal/wiz"` to that file's imports.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestQuietSync|TestCommandFailure' -v`
Expected: FAIL — compile error (`quiet` field / `whiteMode` undefined); the re-sync test fails with nil cmd.

- [ ] **Step 3: Implement**

`internal/ui/tui_state.go`:

```go
type stateSyncResultMsg struct {
	state   wiz.PilotState
	err     error
	elapsed time.Duration
	quiet   bool
}

type pollTickMsg struct{}

// pollCmd schedules the next idle-state heartbeat.
func pollCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg { return pollTickMsg{} })
}

func syncDeviceStateCmd(ip, port string, quiet bool) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		state, err := wiz.GetPilotState(ip, port)
		return stateSyncResultMsg{state: state, err: err, elapsed: time.Since(start), quiet: quiet}
	}
}
```

Model fields: `lastSyncAt time.Time`, `lastSyncOK bool`, `whiteMode bool`.

`internal/ui/tui_update.go`:
- All existing `syncDeviceStateCmd(x, y)` call sites → `syncDeviceStateCmd(x, y, false)`.
- `Init`: append `pollCmd()` to cmds unconditionally (`cmds = append(cmds, pollCmd())`).
- New case:

```go
	case pollTickMsg:
		cmds = append(cmds, pollCmd()) // always reschedule
		if m.state == menuView && m.ip != "" && m.port != "" {
			cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port, true))
		}
		return m, tea.Batch(cmds...)
```

- `stateSyncResultMsg` case: record `m.lastSyncAt = time.Now()`; `m.lastSyncOK = msg.err == nil`. On error: if `msg.quiet` return without status push; else keep existing error status. On success: keep existing field updates, add `m.whiteMode = msg.state.Temp > 0 && strings.TrimSpace(msg.state.ColorHex) == ""`; only push "State synced" when `!msg.quiet`. Set `m.syncingState = false` in all paths as today.
- `commandResultMsg` error branch: after the status push, chain `return m, syncDeviceStateCmd(m.ip, m.port, true)` (also for silent failures — reconcile without narrating).

`renderDashboard` Core block — replace the Color line with mode-aware pair and add Sync line:

```go
	modeLine := fmt.Sprintf("Mode     %s %s",
		lipgloss.NewStyle().Background(mauve).Foreground(base).Bold(true).Padding(0, 1).Render("COLOR"),
		colorSwatch+" "+lipgloss.NewStyle().Foreground(mauve).Render(m.currentColor))
	if m.whiteMode {
		modeLine = fmt.Sprintf("Mode     %s %dK",
			lipgloss.NewStyle().Background(green).Foreground(base).Bold(true).Padding(0, 1).Render("WHITE"),
			m.colorTemp)
	}
	syncLine := "Sync     -"
	if !m.lastSyncAt.IsZero() {
		age := int(time.Since(m.lastSyncAt).Seconds())
		if m.lastSyncOK && age <= 30 {
			syncLine = lipgloss.NewStyle().Foreground(green).Render(fmt.Sprintf("Sync     live · %ds ago", age))
		} else {
			syncLine = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("Sync     stale · %ds ago", age))
		}
	}
```

Core block lines become: Power, Target, alias, modeLine, syncLine. (`time` is already imported in tui_state.go.)

- [ ] **Step 4: Run tests, commit**

Run: `go test ./...` — green.

```bash
git add internal/
git commit -m "feat: 10s sync heartbeat, white/color mode chip, re-sync on command failure"
```

---

### Task 4: CLI subcommands

**Files:**
- Modify: `internal/app/run.go`
- Test: Create `tests/app/cli_test.go` + export a parse helper

**Interfaces:**
- Consumes: `loadRuntimeConfig`, `wiz.SendCommand`, `wiz.GetPilotState`, `wiz.DiscoverDevices`, `wiz.HexToRGB`, `scenes` list is ui-internal — CLI keeps its own tiny scene name→id map (duplicating 12 pairs beats exporting ui internals).
- Produces: `func ParseVerb(args []string) (verb string, rest []string)` — exported for testing; verbs: `on off color temp scene status discover`. Anything else (or empty) → `""` = launch TUI. `Run()` dispatches verbs before flag handling.

- [ ] **Step 1: Write the failing test**

Create `tests/app/cli_test.go`:

```go
package app_test

import (
	"testing"

	"wiz-tui/internal/app"
)

func TestParseVerb(t *testing.T) {
	cases := []struct {
		args []string
		verb string
	}{
		{[]string{}, ""},
		{[]string{"on"}, "on"},
		{[]string{"off"}, "off"},
		{[]string{"color", "#FF0000"}, "color"},
		{[]string{"scene", "party"}, "scene"},
		{[]string{"status"}, "status"},
		{[]string{"--timer", "5"}, ""},
		{[]string{"-v"}, ""},
	}
	for _, tc := range cases {
		verb, _ := app.ParseVerb(tc.args)
		if verb != tc.verb {
			t.Fatalf("ParseVerb(%v) = %q, want %q", tc.args, verb, tc.verb)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/app/ -v`
Expected: FAIL — compile error `undefined: app.ParseVerb`.

- [ ] **Step 3: Implement**

In `internal/app/run.go`:

```go
var cliVerbs = map[string]bool{"on": true, "off": true, "color": true, "temp": true, "scene": true, "status": true, "discover": true}

var sceneIDs = map[string]int{
	"ocean": 1, "romance": 2, "sunset": 3, "party": 4, "fireplace": 5, "cozy": 6,
	"forest": 7, "pastel": 8, "wake-up": 9, "bedtime": 10, "daylight": 12, "focus": 15,
}

// ParseVerb splits a CLI invocation into a known verb and its arguments.
// Unknown first args (flags, empty) mean "no verb": launch the TUI.
func ParseVerb(args []string) (string, []string) {
	if len(args) == 0 || !cliVerbs[args[0]] {
		return "", args
	}
	return args[0], args[1:]
}
```

In `Run()`, after the version check and before the flag declarations:

```go
	if verb, rest := ParseVerb(os.Args[1:]); verb != "" {
		runCLI(verb, rest)
		return
	}
```

Add:

```go
// runCLI executes a one-shot device command and exits.
func runCLI(verb string, args []string) {
	cfg, needsSetup := loadRuntimeConfig()
	if cfg.Port == "" {
		cfg.Port = "38899"
	}
	if needsSetup {
		fmt.Fprintln(os.Stderr, "no configured device - run lumina once to set up, or set WIZ_IP")
		os.Exit(1)
	}

	fail := func(err error) {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", verb, err)
		os.Exit(1)
	}

	switch verb {
	case "on", "off":
		state := verb == "on"
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setState", map[string]interface{}{"state": state}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → power %s\n", cfg.IP, cfg.Port, verb)
	case "color":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina color <#RRGGBB>"))
		}
		r, g, b, err := wiz.HexToRGB(args[0])
		if err != nil {
			fail(fmt.Errorf("invalid hex %q", args[0]))
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → color %s\n", cfg.IP, cfg.Port, args[0])
	case "temp":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina temp <2200-6500>"))
		}
		k, err := strconv.Atoi(args[0])
		if err != nil || k < 2200 || k > 6500 {
			fail(fmt.Errorf("kelvin must be 2200-6500, got %q", args[0]))
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"temp": k}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → temp %dK\n", cfg.IP, cfg.Port, k)
	case "scene":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina scene <name|id>"))
		}
		id, ok := sceneIDs[strings.ToLower(args[0])]
		if !ok {
			if n, err := strconv.Atoi(args[0]); err == nil && n >= 1 && n <= 32 {
				id = n
			} else {
				fail(fmt.Errorf("unknown scene %q", args[0]))
			}
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"sceneId": id}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → scene %s\n", cfg.IP, cfg.Port, args[0])
	case "status":
		targets := [][2]string{}
		for _, d := range cfg.SavedDevices {
			if d.IP != "" {
				port := d.Port
				if port == "" {
					port = cfg.Port
				}
				targets = append(targets, [2]string{d.IP, port})
			}
		}
		if len(targets) == 0 {
			targets = append(targets, [2]string{cfg.IP, cfg.Port})
		}
		for i, t := range targets {
			name := t[0]
			if i < len(cfg.SavedDevices) && cfg.SavedDevices[i].Name != "" {
				name = cfg.SavedDevices[i].Name
			}
			st, err := wiz.GetPilotState(t[0], t[1])
			if err != nil {
				fmt.Printf("%-16s unreachable: %v\n", name, err)
				continue
			}
			power := "off"
			if st.Power {
				power = "on"
			}
			detail := st.ColorHex
			if st.Temp > 0 && st.ColorHex == "" {
				detail = fmt.Sprintf("%dK", st.Temp)
			}
			fmt.Printf("%-16s %s · %d%% · %s\n", name, power, st.Brightness, detail)
		}
	case "discover":
		devices, err := wiz.DiscoverDevices()
		if err != nil {
			fail(err)
		}
		for _, d := range devices {
			fmt.Printf("%-16s %s  %s\n", d.Name, d.IP, d.Mac)
		}
		fmt.Printf("%d device(s)\n", len(devices))
	}
}
```

(Add `"strings"` to run.go imports; already imports the rest.)

- [ ] **Step 4: Run tests + manual, commit**

Run: `go test ./...` — green. Manual: `go run ./internal status` (prints unreachable lines or device states; must not open the TUI), `go run ./internal nonsense` opens TUI normally? No — "nonsense" is not a verb, falls through to flag.Parse which errors on non-flag args? flag.Parse ignores non-flag positional args, TUI opens. Verify: `go run ./internal -v` still prints version.

```bash
git add internal/app/run.go tests/app/
git commit -m "feat: CLI verbs - on off color temp scene status discover"
```

---

### Task 5: Groups

**Files:**
- Modify: `internal/config/config.go` (Group type, Groups field)
- Modify: `internal/ui/tui_state.go` (fanout msg+cmd, group model fields, groupsView, persistConfig, helpers)
- Modify: `internal/ui/tui_update.go` (menu case 2, groupsView case, fanout result case, group-aware send)
- Modify: `internal/ui/tui_view.go` (groups panel, Core Target line shows group)
- Test: append `internal/ui/tui_state_internal_test.go`; fanout black-box test in `tests/ui/` is impractical (unexported cmd) — white-box test in `internal/ui/tui_update_internal_test.go` with loopback UDP servers.

**Interfaces:**
- Consumes: everything prior; `wiz.SendCommand` directly inside fanoutCmd.
- Produces:

```go
// config
type Group struct {
	Name string   `json:"name"`
	Macs []string `json:"macs"`
}
// Config gains: Groups []Group `json:"groups,omitempty"`

// ui
type fanoutResultMsg struct {
	label   string
	ok      int
	failed  []string // device names that failed
	elapsed time.Duration
}
func fanoutCmd(targets [][2]string, names []string, method string, params map[string]interface{}, label string) tea.Cmd
func (m *model) groupTargets() (targets [][2]string, names []string) // resolves active group Macs -> savedDevices IPs
func (m *model) sendToTarget(method string, params map[string]interface{}, successMsg, failPrefix string, onSuccess func(*model)) tea.Cmd
```

Model fields: `groups []config.Group`, `groupCursor int`, `memberCursor int`, `editingGroup int` (-1 = none), `activeGroup string` ("" = single device), plus `groupsView` sessionState.

`sendToTarget` is the group-aware wrapper: when `activeGroup == ""` it returns the existing single `sendCmd(...)`; otherwise it runs `onSuccess` locally on dispatch-model semantics? No — group sends skip per-device onSuccess and return `fanoutCmd(...)` with a label derived from successMsg. Callers in Update replace direct `sendCmd` calls for power/brightness/color/temp/scene with `m.sendToTarget(...)`. Silent previews stay single-target (previewing 5 bulbs at once is hostile).

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/tui_state_internal_test.go`:

```go
func TestGroupTargetsResolvesMacsToIPs(t *testing.T) {
	m := testModel()
	m.savedDevices = []config.SavedDevice{
		{Name: "Desk", IP: "10.0.0.2", Port: "38899", Mac: "aa:aa"},
		{Name: "Shelf", IP: "10.0.0.3", Mac: "bb:bb"},
	}
	m.groups = []config.Group{{Name: "Room", Macs: []string{"aa:aa", "bb:bb", "cc:cc"}}}
	m.activeGroup = "Room"
	targets, names := m.groupTargets()
	if len(targets) != 2 || len(names) != 2 {
		t.Fatalf("expected 2 resolvable members, got %d/%d", len(targets), len(names))
	}
	if targets[0][0] != "10.0.0.2" || targets[0][1] != "38899" {
		t.Fatalf("bad first target: %v", targets[0])
	}
	if targets[1][1] != m.port {
		t.Fatalf("missing member port must fall back to model port, got %q", targets[1][1])
	}
}
```

Append to `internal/ui/tui_update_internal_test.go`:

```go
func TestFanoutCmdAggregates(t *testing.T) {
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, e := srv.ReadFromUDP(buf); e != nil {
				return
			}
		}
	}()
	port := strconv.Itoa(srv.LocalAddr().(*net.UDPAddr).Port)

	targets := [][2]string{{"127.0.0.1", port}, {"127.0.0.1", port}}
	names := []string{"A", "B"}
	msg := fanoutCmd(targets, names, "setState", map[string]interface{}{"state": true}, "Room → on")()
	res, ok := msg.(fanoutResultMsg)
	if !ok {
		t.Fatalf("expected fanoutResultMsg, got %T", msg)
	}
	if res.ok != 2 || len(res.failed) != 0 {
		t.Fatalf("expected 2 ok / 0 failed, got %d/%v", res.ok, res.failed)
	}
}
```

(Imports for that file gain `net`, `strconv`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestGroupTargets|TestFanout' -v`
Expected: FAIL — compile errors (`groups`, `fanoutCmd` undefined).

- [ ] **Step 3: Implement config + fanout + helpers**

`internal/config/config.go`:

```go
// Group names a set of saved devices addressed together.
type Group struct {
	Name string   `json:"name"`
	Macs []string `json:"macs"`
}
```

Config gains `Groups []Group \`json:"groups,omitempty"\``.

`internal/ui/tui_state.go`:

```go
type fanoutResultMsg struct {
	label   string
	ok      int
	failed  []string
	elapsed time.Duration
}

// fanoutCmd sends one command to every target concurrently and aggregates.
func fanoutCmd(targets [][2]string, names []string, method string, params map[string]interface{}, label string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		type result struct {
			idx int
			err error
		}
		results := make(chan result, len(targets))
		for i, t := range targets {
			go func(i int, ip, port string) {
				results <- result{i, wiz.SendCommand(ip, port, method, params)}
			}(i, t[0], t[1])
		}
		res := fanoutResultMsg{label: label}
		for range targets {
			r := <-results
			if r.err != nil {
				res.failed = append(res.failed, names[r.idx])
			} else {
				res.ok++
			}
		}
		sort.Strings(res.failed)
		res.elapsed = time.Since(start)
		return res
	}
}

// groupTargets resolves the active group's MACs to (ip,port) pairs via saved devices.
func (m *model) groupTargets() ([][2]string, []string) {
	var g *config.Group
	for i := range m.groups {
		if m.groups[i].Name == m.activeGroup {
			g = &m.groups[i]
			break
		}
	}
	if g == nil {
		return nil, nil
	}
	byMac := map[string]config.SavedDevice{}
	for _, d := range m.savedDevices {
		byMac[strings.ToLower(strings.TrimSpace(d.Mac))] = d
	}
	var targets [][2]string
	var names []string
	for _, mac := range g.Macs {
		d, ok := byMac[strings.ToLower(strings.TrimSpace(mac))]
		if !ok || d.IP == "" {
			continue // member no longer saved; skipped silently
		}
		port := d.Port
		if port == "" {
			port = m.port
		}
		targets = append(targets, [2]string{d.IP, port})
		names = append(names, d.Name)
	}
	return targets, names
}

// sendToTarget routes a device command to the active group (fan-out) or the single target.
func (m *model) sendToTarget(method string, params map[string]interface{}, successMsg, failPrefix string, onSuccess func(*model)) tea.Cmd {
	if m.activeGroup == "" {
		return sendCmd(m.ip, m.port, method, params, successMsg, failPrefix, onSuccess)
	}
	targets, names := m.groupTargets()
	if len(targets) == 0 {
		return sendCmd(m.ip, m.port, method, params, successMsg, failPrefix, onSuccess)
	}
	return fanoutCmd(targets, names, method, params, m.activeGroup+" → "+successMsg)
}
```

(`sort` joins tui_state.go imports; add `groupsView` to sessionState consts; model fields per Interfaces; `NewModel` sets `groups: cfg.Groups, editingGroup: -1`; `persistConfig` adds `Groups: m.groups,`.)

- [ ] **Step 4: Wire Update**

`internal/ui/tui_update.go`:
- Menu case 2 → `m.state = groupsView`.
- New top-level case:

```go
	case fanoutResultMsg:
		m.recordCommand(msg.elapsed, nil)
		if len(msg.failed) == 0 {
			m = pushStatus(m, statusSuccess, fmt.Sprintf("%s (%d/%d ok)", msg.label, msg.ok, msg.ok))
		} else {
			m.commandFailed++
			m = pushStatus(m, statusError, fmt.Sprintf("%s (%d ok, failed: %s)", msg.label, msg.ok, strings.Join(msg.failed, ", ")))
		}
		return m, nil
```

- Convert the group-eligible senders to `m.sendToTarget(...)`: power toggle, `adjustBrightnessCmd`'s send (change its tail to `return m.sendToTarget("setPilot", ..., func(mm *model){...})`), color grid enter, hex enter, colorTemp closure, scenes apply (Task 2's `apply`). Silent previews (color grid + scenes) stay `sendCmd` single-target.
- groupsView case:

```go
			case groupsView:
				switch msg.String() {
				case "esc", "q":
					if m.editingGroup >= 0 {
						m.editingGroup = -1
					} else {
						m.state = menuView
					}
				case "up", "k":
					if m.editingGroup >= 0 {
						if m.memberCursor > 0 {
							m.memberCursor--
						}
					} else if m.groupCursor > 0 {
						m.groupCursor--
					}
				case "down", "j":
					if m.editingGroup >= 0 {
						if m.memberCursor < len(m.savedDevices)-1 {
							m.memberCursor++
						}
					} else if m.groupCursor < len(m.groups)-1 {
						m.groupCursor++
					}
				case "n":
					m.textInput.CharLimit = 32
					m.textInput.Placeholder = "Group name"
					m.textInput.SetValue("")
					m.textInput.Focus()
					m.state = groupNameView
				case "e":
					if len(m.groups) > 0 {
						m.editingGroup = m.groupCursor
						m.memberCursor = 0
					}
				case " ":
					if m.editingGroup >= 0 && len(m.savedDevices) > 0 {
						mac := strings.ToLower(strings.TrimSpace(m.savedDevices[m.memberCursor].Mac))
						if mac != "" {
							g := &m.groups[m.editingGroup]
							found := -1
							for i, existing := range g.Macs {
								if strings.ToLower(strings.TrimSpace(existing)) == mac {
									found = i
									break
								}
							}
							if found >= 0 {
								g.Macs = append(g.Macs[:found], g.Macs[found+1:]...)
							} else {
								g.Macs = append(g.Macs, mac)
							}
							m.persistConfig()
						}
					}
				case "d":
					if m.editingGroup < 0 && len(m.groups) > 0 {
						name := m.groups[m.groupCursor].Name
						m.groups = append(m.groups[:m.groupCursor], m.groups[m.groupCursor+1:]...)
						if m.activeGroup == name {
							m.activeGroup = ""
						}
						if m.groupCursor >= len(m.groups) && m.groupCursor > 0 {
							m.groupCursor--
						}
						m.persistConfig()
						m = pushStatus(m, statusInfo, "Removed group: "+name)
					}
				case "enter":
					if m.editingGroup >= 0 {
						m.editingGroup = -1
					} else if len(m.groups) > 0 {
						m.activeGroup = m.groups[m.groupCursor].Name
						m = pushStatus(m, statusInfo, "Targeting group: "+m.activeGroup)
						m.state = menuView
					}
				}
```

- New `groupNameView` sessionState (add to consts) handling like saveDeviceNameView:

```go
			case groupNameView:
				switch msg.String() {
				case "esc":
					m.textInput.Blur()
					m.state = groupsView
				case "enter":
					name := strings.TrimSpace(m.textInput.Value())
					if name == "" {
						m = pushStatus(m, statusError, "Group name cannot be empty")
					} else {
						m.groups = append(m.groups, config.Group{Name: name})
						m.groupCursor = len(m.groups) - 1
						m.persistConfig()
						m = pushStatus(m, statusSuccess, "Created group: "+name)
					}
					m.textInput.Blur()
					m.state = groupsView
				default:
					m.textInput, cmd = m.textInput.Update(msg)
					cmds = append(cmds, cmd)
				}
```

- Single-device selection clears the group: in discoveryView enter and savedDevicesView enter, add `m.activeGroup = ""`.

- [ ] **Step 5: View**

`internal/ui/tui_view.go`:

```go
	case groupsView:
		leftPanel = sectionHeader("Groups", "n new · e edit members · Enter targets · d delete") + "\n\n"
		if len(m.groups) == 0 {
			leftPanel += "No groups yet. Press 'n' to create one."
		} else if m.editingGroup >= 0 {
			g := m.groups[m.editingGroup]
			inGroup := map[string]bool{}
			for _, mac := range g.Macs {
				inGroup[strings.ToLower(strings.TrimSpace(mac))] = true
			}
			leftPanel += lipgloss.NewStyle().Foreground(mauve).Bold(true).Render(g.Name) + "  (Space toggles · Enter done)\n\n"
			if len(m.savedDevices) == 0 {
				leftPanel += "No saved devices to add."
			}
			for i, d := range m.savedDevices {
				mark := "[ ]"
				if inGroup[strings.ToLower(strings.TrimSpace(d.Mac))] {
					mark = "[x]"
				}
				style := lipgloss.NewStyle().Foreground(textCol)
				prefix := "  "
				if i == m.memberCursor {
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
					prefix = "> "
				}
				leftPanel += style.Render(fmt.Sprintf("%s%s %s", prefix, mark, d.Name)) + lipgloss.NewStyle().Foreground(subtext).Render("  "+d.IP) + "\n"
			}
		} else {
			for i, g := range m.groups {
				style := lipgloss.NewStyle().Foreground(textCol)
				prefix := "  "
				if i == m.groupCursor {
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
					prefix = "> "
				}
				active := ""
				if g.Name == m.activeGroup {
					active = lipgloss.NewStyle().Foreground(green).Render("  targeted")
				}
				leftPanel += style.Render(fmt.Sprintf("%s%-16s", prefix, g.Name)) +
					lipgloss.NewStyle().Foreground(subtext).Render(fmt.Sprintf("%d member(s)", len(g.Macs))) + active + "\n"
			}
		}
```

`renderDashboard` Target line: when `m.activeGroup != ""`, show `fmt.Sprintf("Target   group: %s", m.activeGroup)` instead of ip:port.

- [ ] **Step 6: Run tests, commit**

Run: `go test ./...` — green.

```bash
git add internal/ tests/
git commit -m "feat: device groups with concurrent fan-out and aggregated status"
```

---

### Task 6: Final verification

- [ ] `go vet ./... && go test ./...` — clean and green.
- [ ] `go build -o /dev/null ./internal` — builds.
- [ ] `go run ./internal -v` — version prints.
- [ ] `go run ./internal status` — CLI path works headless (unreachable lines are fine without bulbs).
- [ ] Manual TUI smoke: menu shows 13 entries; Theme view switches palette live; Scenes/Groups views open; dashboard shows Mode + Sync lines.
- [ ] `grep -rn "wiz.SendCommand" internal/ui/` — matches only inside `sendCmd` and `fanoutCmd` in `tui_state.go`.
