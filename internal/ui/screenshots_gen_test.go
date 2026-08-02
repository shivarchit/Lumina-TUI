package ui

// Screenshot generator: renders real View() output and converts the ANSI to
// SVG for the README. Run with:
//
//	GEN_SCREENSHOTS=$(pwd)/docs/screenshots go test ./internal/ui -run TestGenerateScreenshots
//
// ponytail: bespoke SGR->SVG converter (~100 lines) because no capture tool
// (vhs/freeze) is available; swap for freeze if it ever lands in CI.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shivarchit/Lumina-TUI/pkg/config"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestGenerateScreenshots(t *testing.T) {
	outDir := os.Getenv("GEN_SCREENSHOTS")
	if outDir == "" {
		t.Skip("set GEN_SCREENSHOTS=<output dir> to generate")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	lipgloss.SetColorProfile(termenv.TrueColor)
	applyTheme("mocha")

	cfg := config.Config{
		IP: "192.168.1.42", Port: "38899",
		SavedDevices: []config.SavedDevice{
			{Name: "Desk Lamp", IP: "192.168.1.42", Port: "38899", Mac: "a8:bb:50:11:22:33"},
			{Name: "Shelf Strip", IP: "192.168.1.43", Port: "38899", Mac: "a8:bb:50:44:55:66"},
			{Name: "Corner Bulb", IP: "192.168.1.44", Port: "38899", Mac: "a8:bb:50:77:88:99"},
		},
		Groups: []config.Group{
			{Name: "Living Room", Macs: []string{"a8:bb:50:11:22:33", "a8:bb:50:44:55:66", "a8:bb:50:77:88:99"}},
			{Name: "Bedroom", Macs: []string{"a8:bb:50:44:55:66"}},
		},
	}

	base := NewModel(cfg, false)
	base.windowWidth = 130
	base.brightness = 72
	base.brightnessHistory = []int{55, 60, 60, 65, 70, 80, 75, 70, 72, 72}
	base.commandTotal = 42
	base.commandFailed = 1
	base.commandLatencyMs = []int{20, 18, 25, 16, 19, 22, 17, 18, 21, 18}
	base.whiteMode = true
	base.colorTemp = 4000
	base.lastSyncAt = time.Now().Add(-2 * time.Second)
	base.lastSyncOK = true
	base.syncingState = false

	shots := map[string]func() model{
		"main": func() model { return base },
		"scenes": func() model {
			m := base
			m.state = scenesView
			m.sceneCursor = 0
			m.lastScene = 1
			return m
		},
		"themes": func() model {
			m := base
			m.state = themesView
			m.themeCursor = 0
			return m
		},
		"groups": func() model {
			m := base
			m.state = groupsView
			m.activeGroup = "Living Room"
			return m
		},
	}

	for name, build := range shots {
		svg := ansiToSVG(build().View())
		path := filepath.Join(outDir, name+".svg")
		if err := os.WriteFile(path, []byte(svg), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

const (
	cellW      = 7.8
	cellH      = 17.0
	padding    = 18.0
	fontSize   = 13
	defaultFG  = "#CDD6F4"
	terminalBG = "#11111B"
)

type sgrState struct {
	fg, bg string
	bold   bool
}

// ansiToSVG converts SGR-colored terminal output into an SVG image.
func ansiToSVG(ansi string) string {
	lines := strings.Split(ansi, "\n")
	maxCols := 0
	var body strings.Builder

	for row, line := range lines {
		col := 0
		st := sgrState{fg: defaultFG}
		i := 0
		runes := []rune(line)
		var runText strings.Builder
		runStart := 0
		runState := st

		flush := func() {
			text := runText.String()
			if text == "" {
				return
			}
			x := padding + float64(runStart)*cellW
			y := padding + float64(row)*cellH
			width := float64(len([]rune(text))) * cellW
			if runState.bg != "" {
				fmt.Fprintf(&body, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n",
					x, y, width, cellH, runState.bg)
			}
			if strings.TrimSpace(text) != "" {
				weight := ""
				if runState.bold {
					weight = ` font-weight="bold"`
				}
				fmt.Fprintf(&body, `<text x="%.1f" y="%.1f" fill="%s"%s xml:space="preserve">%s</text>`+"\n",
					x, y+cellH-4.5, runState.fg, weight, xmlEscape(text))
			}
			runText.Reset()
		}

		for i < len(runes) {
			r := runes[i]
			if r == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
				end := i + 2
				for end < len(runes) && runes[end] != 'm' {
					end++
				}
				if end < len(runes) {
					flush()
					st = applySGR(st, string(runes[i+2:end]))
					runStart = col
					runState = st
					i = end + 1
					continue
				}
			}
			if runText.Len() == 0 {
				runStart = col
				runState = st
			}
			runText.WriteRune(r)
			col++
			i++
		}
		flush()
		if col > maxCols {
			maxCols = col
		}
	}

	w := padding*2 + float64(maxCols)*cellW
	h := padding*2 + float64(len(lines))*cellH
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="SF Mono, Menlo, Consolas, monospace" font-size="%d">
<rect width="100%%" height="100%%" rx="10" fill="%s"/>
%s</svg>
`, w, h, fontSize, terminalBG, body.String())
}

func applySGR(st sgrState, params string) sgrState {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "", "0":
			st = sgrState{fg: defaultFG}
		case "1":
			st.bold = true
		case "22":
			st.bold = false
		case "39":
			st.fg = defaultFG
		case "49":
			st.bg = ""
		case "38", "48":
			if i+4 < len(parts) && parts[i+1] == "2" {
				r, _ := strconv.Atoi(parts[i+2])
				g, _ := strconv.Atoi(parts[i+3])
				b, _ := strconv.Atoi(parts[i+4])
				c := fmt.Sprintf("#%02X%02X%02X", r, g, b)
				if parts[i] == "38" {
					st.fg = c
				} else {
					st.bg = c
				}
				i += 4
			}
		}
	}
	return st
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
