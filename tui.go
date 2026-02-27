package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	setupView sessionState = iota // Added setup state
	menuView
	colorPickerView
	hexInputView
	brightnessView
	timerInputView
	discoveryView
	helpView
)

type timerFinishedMsg struct{}

var (
	mauve   = lipgloss.Color("#CBA6F7")
	blue    = lipgloss.Color("#89B4FA")
	green   = lipgloss.Color("#A6E3A1")
	textCol = lipgloss.Color("#CDD6F4")
	subtext = lipgloss.Color("#6C7086")
	surface = lipgloss.Color("#313244")
	base    = lipgloss.Color("#1E1E2E")
)

var colorPalette = []struct{ name, hex string }{
	{"Warm", "#FFB56B"}, {"Day", "#FFE4CE"}, {"Cool", "#E0F7FA"},
	{"Ruby", "#FF0033"}, {"Rose", "#FF66CC"}, {"Pink", "#FFB6C1"},
	{"Peach", "#FF9966"}, {"Orng", "#FF8C00"}, {"Gold", "#FFD700"},
	{"Lime", "#32CD32"}, {"Mint", "#98FF98"}, {"Emrld", "#00FF00"},
	{"Teal", "#008080"}, {"Aqua", "#00FFFF"}, {"Sky", "#87CEEB"},
	{"Ocean", "#006994"}, {"Blue", "#0000FF"}, {"Navy", "#000080"},
	{"Lvndr", "#E6E6FA"}, {"Prple", "#800080"}, {"Mgnta", "#FF00FF"},
}

type model struct {
	state        sessionState
	setupStep    int // Tracks IP vs Port
	choices      []string
	icons        []string
	cursor       int
	colorCursor  int
	status       string
	ip, port     string
	isOn         bool
	currentColor string
	brightness   int
	textInput    textinput.Model
	spinner      spinner.Model
	timerActive  bool
	// detachedTimer indicates we've spawned an external worker; when the
	// local timer fires we should avoid sending an extra off command.
	detachedTimer     bool
	discoveredDevices []Device
	deviceCursor      int
}

func initialModel(ip, port string, needsSetup bool) model {
	ti := textinput.New()
	ti.CharLimit = 15
	ti.Width = 20
	ti.PromptStyle = lipgloss.NewStyle().Foreground(mauve)
	ti.TextStyle = lipgloss.NewStyle().Foreground(textCol)

	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = lipgloss.NewStyle().Foreground(blue).Bold(true)

	state := menuView
	if needsSetup {
		state = setupView
		ti.Placeholder = "e.g. 192.168.1.15"
		ti.Focus()
	}

	return model{
		state:             state,
		setupStep:         0,
		choices:           []string{"Toggle Power", "Color Grid", "Hex Colors", "Brightness", "Sleep Timer", "Discover Devices", "Help", "Exit"},
		icons:             []string{"⚡", "🎨", "✍️", "☀️", "⏱️", "🔍", "❓", "🚪"},
		status:            "Ready.",
		ip:                ip,
		port:              port,
		isOn:              false,
		currentColor:      "#CBA6F7",
		brightness:        100,
		textInput:         ti,
		spinner:           s,
		discoveredDevices: []Device{},
		deviceCursor:      0,
	}
}

func startTimer(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return timerFinishedMsg{}
	}
}

// startDetachedTimer launches a second copy of the current binary with
// `--timer` arguments.  Because the child is started with Setpgid the
// process will continue running after the parent (the TUI) exits, allowing
// the sleep command to still be sent even if the user quits the interface.
func startDetachedTimer(mins int, ip, port string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"--timer", strconv.Itoa(mins), "--ip", ip, "--port", port, "--off"}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// detach from parent process so it won't be killed when the TUI exits
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.timerActive {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	case timerFinishedMsg:
		m.timerActive = false
		m.isOn = false
		if m.detachedTimer {
			// the worker process will already have sent the command
			m.status = "Timer finished (handled in background)"
		} else {
			if err := sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": false}); err != nil {
				m.status = fmt.Sprintf("Timer finished. Power off failed: %v", err)
			} else {
				m.status = "Timer finished. Power off."
			}
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Handle Setup Logic Before Main Menu
		if m.state == setupView {
			switch msg.String() {
			case "enter":
				if m.setupStep == 0 {
					m.ip = m.textInput.Value()
					if m.ip == "" {
						m.ip = "192.168.1.2"
					} // Fallback
					m.setupStep = 1
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. 38899"
				} else {
					m.port = m.textInput.Value()
					if m.port == "" {
						m.port = "38899"
					} // Fallback

					// Save Config
					cfg := Config{IP: m.ip, Port: m.port}
					data, _ := json.Marshal(cfg)
					_ = os.WriteFile(getConfigPath(), data, 0644)

					m.state = menuView
					m.textInput.Blur()
					m.textInput.SetValue("")
					m.status = "Config Saved!"
				}
			case "esc":
				return m, tea.Quit
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Original Menu Logic
		switch m.state {
		case menuView:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			case "enter", " ":
				switch m.cursor {
				case 0:
					m.isOn = !m.isOn
					if err := sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": m.isOn}); err != nil {
						m.status = fmt.Sprintf("Power toggle failed: %v", err)
						m.isOn = !m.isOn // Revert state on error
					} else {
						if m.isOn {
							m.status = "Power: ON"
						} else {
							m.status = "Power: OFF"
						}
					}
				case 1:
					m.state = colorPickerView
				case 2:
					m.state = hexInputView
					m.textInput.Placeholder = "#CBA6F7"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 3:
					m.state = brightnessView
				case 4:
					m.state = timerInputView
					m.textInput.Placeholder = "Mins (e.g. 15)"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 5:
					m.state = discoveryView
					m.status = "Discovering devices..."
					// Note: Device discovery will be handled in a separate command
				case 6:
					m.state = helpView
				case 7:
					return m, tea.Quit
				}
			}

		case colorPickerView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "up", "k":
				if m.colorCursor >= 3 {
					m.colorCursor -= 3
				}
			case "down", "j":
				if m.colorCursor < len(colorPalette)-3 {
					m.colorCursor += 3
				}
			case "left", "h":
				if m.colorCursor > 0 {
					m.colorCursor--
				}
			case "right", "l":
				if m.colorCursor < len(colorPalette)-1 {
					m.colorCursor++
				}
			case "enter":
				selectedHex := colorPalette[m.colorCursor].hex
				r, g, b, _ := hexToRGB(selectedHex)
				if err := sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness}); err != nil {
					m.status = fmt.Sprintf("Color change failed: %v", err)
				} else {
					m.currentColor = selectedHex
					m.isOn = true
					m.status = "Color: " + colorPalette[m.colorCursor].name
				}
				m.state = menuView
			}

		case hexInputView:
			switch msg.String() {
			case "esc":
				m.state = menuView
			case "enter":
				val := m.textInput.Value()
				r, g, b, err := hexToRGB(val)
				if err != nil {
					m.status = "Err: Invalid Hex"
				} else {
					if cmdErr := sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness}); cmdErr != nil {
						m.status = fmt.Sprintf("Color change failed: %v", cmdErr)
					} else {
						m.currentColor = val
						m.isOn = true
						m.status = fmt.Sprintf("Color: %s", val)
					}
				}
				m.state = menuView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}

		case brightnessView:
			switch msg.String() {
			case "esc", "q", "enter":
				m.state = menuView
			case "left", "h":
				if m.brightness > 10 {
					m.brightness -= 10
					if err := sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness}); err != nil {
						m.status = fmt.Sprintf("Brightness change failed: %v", err)
						m.brightness += 10 // Revert on error
					} else {
						m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
					}
				}
			case "right", "l":
				if m.brightness < 100 {
					m.brightness += 10
					if err := sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness}); err != nil {
						m.status = fmt.Sprintf("Brightness change failed: %v", err)
						m.brightness -= 10 // Revert on error
					} else {
						m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
					}
				}
			}

		case timerInputView:
			switch msg.String() {
			case "esc":
				m.state = menuView
			case "enter":
				val := m.textInput.Value()
				mins, err := strconv.Atoi(val)
				if err == nil && mins > 0 {
					m.timerActive = true
					m.detachedTimer = true
					m.status = fmt.Sprintf("Sleep in %dm", mins)
					cmds = append(cmds, startTimer(time.Duration(mins)*time.Minute), m.spinner.Tick)
					// spawn an independent worker process so the timer survives closing
					if err := startDetachedTimer(mins, m.ip, m.port); err != nil {
						m.status = fmt.Sprintf("Timer spawn failed: %v", err)
					}
				}
				m.state = menuView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
			switch msg.String() {
			case "esc", "q", "enter":
				m.state = menuView
			}

		case discoveryView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "r": // Refresh discovery
				m.status = "Discovering devices..."
				// Discovery command would be added here
			case "up", "k":
				if m.deviceCursor > 0 {
					m.deviceCursor--
				}
			case "down", "j":
				if m.deviceCursor < len(m.discoveredDevices)-1 {
					m.deviceCursor++
				}
			case "enter":
				if len(m.discoveredDevices) > 0 {
					selectedDevice := m.discoveredDevices[m.deviceCursor]
					m.ip = selectedDevice.IP
					m.status = fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.IP)
					m.state = menuView
				}
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// First-run Setup Screen Rendering
	if m.state == setupView {
		prompt := "Enter WiZ Device IP Address:"
		if m.setupStep == 1 {
			prompt = "Enter UDP Port (Default 38899):"
		}

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mauve).
			Padding(2, 4).
			Width(42).
			Align(lipgloss.Center)

		content := fmt.Sprintf("%s\n\n%s\n\n%s",
			lipgloss.NewStyle().Bold(true).Foreground(mauve).Render("FIRST-TIME SETUP"),
			prompt,
			m.textInput.View())

		return "\n" + box.Render(content) + "\n"
	}

	// Original pristine UI Rendering
	borderColor := surface
	if m.isOn {
		borderColor = mauve
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(42).
		Height(15)

	itemStyle := lipgloss.NewStyle().Foreground(subtext).PaddingLeft(1)
	selectedStyle := lipgloss.NewStyle().Foreground(mauve).Bold(true).PaddingLeft(1)

	var leftPanel string
	switch m.state {
	case menuView:
		for i, choice := range m.choices {
			icon := m.icons[i]
			if m.cursor == i {
				leftPanel += selectedStyle.Render(fmt.Sprintf("┃ %s  %s", icon, choice)) + "\n"
			} else {
				leftPanel += itemStyle.Render(fmt.Sprintf("  %s  %s", icon, choice)) + "\n"
			}
		}
	case colorPickerView:
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Select Color") + "\n\n"
		for i, c := range colorPalette {
			text := c.name
			bg := lipgloss.Color(c.hex)
			fg := lipgloss.Color("#11111B")
			if m.colorCursor == i {
				text = "▶ " + text
			} else {
				text = "  " + text
			}
			block := lipgloss.NewStyle().Background(bg).Foreground(fg).Width(11).Align(lipgloss.Center).Render(text)
			leftPanel += block + " "
			if (i+1)%3 == 0 {
				leftPanel += "\n"
			}
		}
	case hexInputView:
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Input Hex Code") + "\n\n" + m.textInput.View()
	case brightnessView:
		bars := m.brightness / 10
		slider := strings.Repeat("━", bars) + "┫" + strings.Repeat("┈", 10-bars)
		coloredSlider := lipgloss.NewStyle().Foreground(mauve).Render(slider)
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Brightness Control") + fmt.Sprintf("\n\n%s %d%%", coloredSlider, m.brightness)
	case timerInputView:
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Sleep Timer") + "\n\n" + m.textInput.View()
	case helpView:
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Help & Controls") + "\n\n" +
			lipgloss.NewStyle().Foreground(textCol).Render("Navigation:\n") +
			"↑↓/jk  - Move cursor\n" +
			"Enter   - Select/Confirm\n" +
			"Esc     - Cancel/Back\n" +
			"q/Ctrl+C- Quit\n\n" +
			lipgloss.NewStyle().Foreground(textCol).Render("Features:\n") +
			"⚡ Power - Toggle on/off\n" +
			"🎨 Colors- 24 preset colors\n" +
			"✍️ Hex   - Custom hex codes\n" +
			"☀️ Bright- Adjust dimming\n" +
			"⏱️ Timer - Auto power off\n" +
			"🔍 Discov- Find devices\n" +
			"❓ Help  - This screen"
	case discoveryView:
		leftPanel = lipgloss.NewStyle().Foreground(blue).Render("Device Discovery") + "\n\n"
		if len(m.discoveredDevices) == 0 {
			leftPanel += "No devices found.\n\nPress 'r' to refresh."
		} else {
			leftPanel += "Found devices:\n\n"
			for i, device := range m.discoveredDevices {
				prefix := "  "
				if i == m.deviceCursor {
					prefix = "▶ "
				}
				leftPanel += fmt.Sprintf("%s%s (%s)\n", prefix, device.Name, device.IP)
			}
			leftPanel += "\nEnter to select, 'r' to refresh"
		}
	}

	powerIcon := "⏺"
	powerColor := subtext
	if m.isOn {
		powerColor = green
	}

	rightPanel := fmt.Sprintf("%s\n\nTarget:\n%s:%s\n\nAction:\n%s",
		lipgloss.NewStyle().Foreground(powerColor).Render(powerIcon+" Power"),
		m.ip, m.port, m.status)

	if m.timerActive {
		rightPanel += fmt.Sprintf("\n\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(blue).Render("Timer Active"))
	}

	leftBox := panelStyle.Render(leftPanel)
	rightBox := panelStyle.Render(rightPanel)

	mainUI := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	modeStr := " NORMAL "
	modeBg := blue
	if m.state != menuView {
		modeStr = " INSERT "
		modeBg = mauve
	}

	modeBadge := lipgloss.NewStyle().Background(modeBg).Foreground(base).Bold(true).Render(modeStr)
	infoBadge := lipgloss.NewStyle().Background(surface).Foreground(textCol).Padding(0, 1).Render("Lumina")
	versionBadge := lipgloss.NewStyle().Background(base).Foreground(subtext).Padding(0, 1).Render(Version)

	statusBar := lipgloss.JoinHorizontal(lipgloss.Top, modeBadge, infoBadge, versionBadge)

	return "\n" + mainUI + "\n" + statusBar + "\n"
}
