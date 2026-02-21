package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	menuView sessionState = iota
	colorPickerView
	hexInputView
	brightnessView
	timerInputView
)

type timerFinishedMsg struct{}

// Sleek Catppuccin-inspired Palette
var (
	mauve   = lipgloss.Color("#CBA6F7")
	blue    = lipgloss.Color("#89B4FA")
	green   = lipgloss.Color("#A6E3A1")
	red     = lipgloss.Color("#F38BA8")
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
}

func initialModel(ip, port string) model {
	ti := textinput.New()
	ti.CharLimit = 10
	ti.Width = 20
	ti.PromptStyle = lipgloss.NewStyle().Foreground(mauve)
	ti.TextStyle = lipgloss.NewStyle().Foreground(textCol)

	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = lipgloss.NewStyle().Foreground(blue).Bold(true)

	return model{
		state:        menuView,
		choices:      []string{"Toggle Power", "Color Grid", "Hex Color", "Brightness", "Sleep Timer", "Exit"},
		icons:        []string{"⚡", "🎨", "✍️", "☀️", "⏱️", "🚪"},
		status:       "Ready.",
		ip:           ip,
		port:         port,
		isOn:         true,
		currentColor: "#CBA6F7",
		brightness:   100,
		textInput:    ti,
		spinner:      s,
	}
}

func startTimer(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return timerFinishedMsg{}
	}
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
		sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": false})
		m.status = "Timer finished. Power off."
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case menuView:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 { m.cursor-- }
			case "down", "j":
				if m.cursor < len(m.choices)-1 { m.cursor++ }
			case "enter", " ":
				switch m.cursor {
				case 0:
					m.isOn = !m.isOn
					sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": m.isOn})
					if m.isOn { m.status = "Power: ON" } else { m.status = "Power: OFF" }
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
					return m, tea.Quit
				}
			}

		case colorPickerView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "up", "k":
				if m.colorCursor >= 3 { m.colorCursor -= 3 }
			case "down", "j":
				if m.colorCursor < len(colorPalette)-3 { m.colorCursor += 3 }
			case "left", "h":
				if m.colorCursor > 0 { m.colorCursor-- }
			case "right", "l":
				if m.colorCursor < len(colorPalette)-1 { m.colorCursor++ }
			case "enter":
				selectedHex := colorPalette[m.colorCursor].hex
				r, g, b, _ := hexToRGB(selectedHex)
				sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				m.currentColor = selectedHex
				m.isOn = true
				m.status = "Color: " + colorPalette[m.colorCursor].name
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
					sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
					m.currentColor = val
					m.isOn = true
					m.status = fmt.Sprintf("Color: %s", val)
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
				if m.brightness > 10 { m.brightness -= 10 }
				sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness})
				m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
			case "right", "l":
				if m.brightness < 100 { m.brightness += 10 }
				sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness})
				m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
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
					m.status = fmt.Sprintf("Sleep in %dm", mins)
					cmds = append(cmds, startTimer(time.Duration(mins)*time.Minute), m.spinner.Tick)
				}
				m.state = menuView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// Styling based on device state
	borderColor := surface
	if m.isOn {
		borderColor = mauve
	}

	// Increase width slightly and set explicit height to ensure borders close
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(42).  // Increased width to prevent clipping
		Height(15)

	// List Styles
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
			if m.colorCursor == i { text = "▶ " + text } else { text = "  " + text }
			block := lipgloss.NewStyle().Background(bg).Foreground(fg).Width(11).Align(lipgloss.Center).Render(text)
			leftPanel += block + " "
			if (i+1)%3 == 0 { leftPanel += "\n" }
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
	}

	powerIcon := "⏺"
	powerColor := subtext
	if m.isOn { powerColor = green }
	
	rightPanel := fmt.Sprintf("%s\n\nTarget:\n%s:%s\n\nAction:\n%s", 
		lipgloss.NewStyle().Foreground(powerColor).Render(powerIcon+" Power"), 
		m.ip, m.port, m.status)

	if m.timerActive {
		rightPanel += fmt.Sprintf("\n\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(blue).Render("Timer Active"))
	}

	// This ensures the content is aligned at the TOP LEFT of the box
	leftBox := panelStyle.Render(leftPanel)
	rightBox := panelStyle.Render(rightPanel)

	mainUI := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	// Bottom Bar
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