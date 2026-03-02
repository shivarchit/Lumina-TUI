package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wiz-tui/internal/config"
	"wiz-tui/internal/wiz"
)

type sessionState int

const (
	setupView sessionState = iota
	menuView
	colorPickerView
	hexInputView
	brightnessView
	timerInputView
	discoveryView
	savedDevicesView
	saveDeviceNameView
	helpView
)

type timerFinishedMsg struct{}

type discoveryResultMsg struct {
	devices []wiz.Device
	err     error
	elapsed time.Duration
}

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
	state         sessionState
	setupStep     int
	choices       []string
	icons         []string
	cursor        int
	colorCursor   int
	status        string
	ip, port      string
	isOn          bool
	currentColor  string
	brightness    int
	textInput     textinput.Model
	spinner       spinner.Model
	timerActive   bool
	detachedTimer bool

	discovering        bool
	discoveredDevices  []wiz.Device
	deviceCursor       int
	savedDevices       []config.SavedDevice
	savedDeviceCursor  int
	pendingSaveDevice  wiz.Device
	discoveryRuns      int
	lastDiscoveryCount int
	lastDiscoveryMs    int

	commandTotal       int
	commandFailed      int
	brightnessHistory  []int
	commandLatencyMs   []int
	discoveryLatencyMs []int
}

// NewModel creates the first TUI model from runtime config.
func NewModel(cfg config.Config, needsSetup bool) model {
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
		state:              state,
		setupStep:          0,
		choices:            []string{"Toggle Power", "Color Grid", "Hex Colors", "Brightness", "Sleep Timer", "Discover Devices", "Saved Devices", "Help", "Exit"},
		icons:              []string{"⚡", "🎨", "✍️", "☀️", "⏱️", "🔍", "💾", "❓", "🚪"},
		status:             "Ready.",
		ip:                 cfg.IP,
		port:               cfg.Port,
		isOn:               true,
		currentColor:       "#CBA6F7",
		brightness:         100,
		textInput:          ti,
		spinner:            s,
		discoveredDevices:  []wiz.Device{},
		deviceCursor:       0,
		savedDevices:       cfg.SavedDevices,
		savedDeviceCursor:  0,
		brightnessHistory:  []int{100},
		commandLatencyMs:   []int{},
		discoveryLatencyMs: []int{},
	}
}

// persistConfig saves current target and saved devices to config storage.
func (m *model) persistConfig() {
	_ = config.Save(config.Config{IP: m.ip, Port: m.port, SavedDevices: m.savedDevices})
}

// upsertSavedDevice inserts or updates a saved device record keyed by MAC.
func (m *model) upsertSavedDevice(device config.SavedDevice) {
	if strings.TrimSpace(device.Mac) == "" {
		return
	}
	device.Mac = strings.ToLower(strings.TrimSpace(device.Mac))

	for index := range m.savedDevices {
		if strings.ToLower(strings.TrimSpace(m.savedDevices[index].Mac)) == device.Mac {
			m.savedDevices[index] = device
			return
		}
	}
	m.savedDevices = append(m.savedDevices, device)
}

// applySavedNamesToDiscovered overlays user-saved names onto discovered devices by MAC.
func (m *model) applySavedNamesToDiscovered() {
	if len(m.discoveredDevices) == 0 || len(m.savedDevices) == 0 {
		return
	}

	savedNameByMAC := map[string]string{}
	for _, saved := range m.savedDevices {
		mac := strings.ToLower(strings.TrimSpace(saved.Mac))
		name := strings.TrimSpace(saved.Name)
		if mac == "" || name == "" {
			continue
		}
		savedNameByMAC[mac] = name
	}

	for index := range m.discoveredDevices {
		mac := strings.ToLower(strings.TrimSpace(m.discoveredDevices[index].Mac))
		if mac == "" {
			continue
		}
		if savedName, ok := savedNameByMAC[mac]; ok {
			m.discoveredDevices[index].Name = savedName
		}
	}
}

// currentTargetSavedName returns a saved alias for the active target device.
func (m model) currentTargetSavedName() string {
	activeMAC := ""
	for _, device := range m.discoveredDevices {
		if device.IP == m.ip {
			activeMAC = strings.ToLower(strings.TrimSpace(device.Mac))
			break
		}
	}

	if activeMAC != "" {
		for _, saved := range m.savedDevices {
			savedMAC := strings.ToLower(strings.TrimSpace(saved.Mac))
			if savedMAC != "" && savedMAC == activeMAC {
				return saved.Name
			}
		}
	}

	for _, saved := range m.savedDevices {
		if saved.IP == m.ip {
			return saved.Name
		}
	}

	return ""
}

// deleteSavedDevice removes a saved device at the selected cursor position.
func (m *model) deleteSavedDevice() {
	if len(m.savedDevices) == 0 || m.savedDeviceCursor < 0 || m.savedDeviceCursor >= len(m.savedDevices) {
		return
	}
	m.savedDevices = append(m.savedDevices[:m.savedDeviceCursor], m.savedDevices[m.savedDeviceCursor+1:]...)
	if m.savedDeviceCursor >= len(m.savedDevices) && m.savedDeviceCursor > 0 {
		m.savedDeviceCursor--
	}
}

// startTimer returns a command that emits when a timer duration has elapsed.
func startTimer(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return timerFinishedMsg{}
	}
}

// discoverDevicesCmd runs network discovery asynchronously.
func discoverDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		devices, err := wiz.DiscoverDevices()
		return discoveryResultMsg{devices: devices, err: err, elapsed: time.Since(start)}
	}
}

// startDetachedTimer launches a detached worker process for timer actions.
func startDetachedTimer(mins int, ip, port string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"--timer", strconv.Itoa(mins), "--ip", ip, "--port", port, "--off"}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setDetachedProcessAttrs(cmd)
	return cmd.Start()
}

// recordCommand updates command success and latency telemetry.
func (m *model) recordCommand(latency time.Duration, err error) {
	m.commandTotal++
	if err != nil {
		m.commandFailed++
	}
	m.commandLatencyMs = appendBounded(m.commandLatencyMs, int(latency.Milliseconds()), 30)
}

// appendBounded appends to a history slice and keeps it capped.
func appendBounded(history []int, value, maxLen int) []int {
	if maxLen <= 0 {
		return history
	}
	history = append(history, value)
	if len(history) > maxLen {
		history = history[len(history)-maxLen:]
	}
	return history
}

// sparkline renders a compact chart from numeric samples.
func sparkline(values []int, width int) string {
	if width <= 0 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat("·", width)
	}

	blocks := []rune("▁▂▃▄▅▆▇█")
	start := 0
	if len(values) > width {
		start = len(values) - width
	}
	samples := values[start:]

	maxVal := 1
	for _, value := range samples {
		if value > maxVal {
			maxVal = value
		}
	}

	var b strings.Builder
	for _, value := range samples {
		idx := (value * (len(blocks) - 1)) / maxVal
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}

	if len(samples) < width {
		return strings.Repeat("·", width-len(samples)) + b.String()
	}
	return b.String()
}

// bar renders a fixed-width filled bar for a value range.
func bar(value, max, width int) string {
	if width <= 0 {
		return ""
	}
	if max <= 0 {
		max = 1
	}
	if value < 0 {
		value = 0
	}
	if value > max {
		value = max
	}
	filled := (value * width) / max
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// metricBlock renders a bordered dashboard card.
func metricBlock(title string, lines []string, accent lipgloss.Color, width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(textCol)
	blockStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(width)

	content := titleStyle.Render(title) + "\n" + bodyStyle.Render(strings.Join(lines, "\n"))
	return blockStyle.Render(content)
}

// sectionHeader renders a labeled panel header with optional subtitle.
func sectionHeader(title, subtitle string) string {
	titleBadge := lipgloss.NewStyle().
		Background(blue).
		Foreground(base).
		Bold(true).
		Padding(0, 1).
		Render(" " + title + " ")

	if subtitle == "" {
		return titleBadge
	}

	sub := lipgloss.NewStyle().Foreground(subtext).Render(" " + subtitle)
	return lipgloss.JoinHorizontal(lipgloss.Top, titleBadge, sub)
}

// renderDashboard builds the right-side stats dashboard.
func (m model) renderDashboard() string {
	powerState := "OFF"
	powerStyle := lipgloss.NewStyle().Foreground(red)
	if m.isOn {
		powerState = "ON"
		powerStyle = lipgloss.NewStyle().Foreground(green)
	}

	successCount := m.commandTotal - m.commandFailed
	successRate := 100
	if m.commandTotal > 0 {
		successRate = (successCount * 100) / m.commandTotal
	}

	latestCmdLatency := 0
	if len(m.commandLatencyMs) > 0 {
		latestCmdLatency = m.commandLatencyMs[len(m.commandLatencyMs)-1]
	}

	discoveryRate := 0
	if m.discoveryRuns > 0 {
		discoveryRate = (m.lastDiscoveryCount * 100) / 10
		if discoveryRate > 100 {
			discoveryRate = 100
		}
	}

	targetAlias := m.currentTargetSavedName()
	aliasLine := "Alias    -"
	if strings.TrimSpace(targetAlias) != "" {
		aliasLine = fmt.Sprintf("Alias    %s", targetAlias)
	}

	core := metricBlock("Core", []string{
		fmt.Sprintf("Power    %s", powerStyle.Bold(true).Render(powerState)),
		fmt.Sprintf("Target   %s:%s", m.ip, m.port),
		aliasLine,
		fmt.Sprintf("Color    %s", lipgloss.NewStyle().Foreground(mauve).Render(m.currentColor)),
	}, blue, 34)

	brightnessBlock := metricBlock("Brightness", []string{
		lipgloss.NewStyle().Foreground(mauve).Render(bar(m.brightness, 100, 22)),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.brightnessHistory, 22)),
		fmt.Sprintf("Level    %d%%", m.brightness),
	}, mauve, 34)

	commandBlock := metricBlock("Command Health", []string{
		lipgloss.NewStyle().Foreground(green).Render(bar(successRate, 100, 22)),
		fmt.Sprintf("OK/Fail  %d/%d", successCount, m.commandFailed),
		fmt.Sprintf("Latency  %dms", latestCmdLatency),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.commandLatencyMs, 22)),
	}, green, 34)

	discoveryBlock := metricBlock("Discovery", []string{
		lipgloss.NewStyle().Foreground(mauve).Render(bar(discoveryRate, 100, 22)),
		fmt.Sprintf("Runs     %d", m.discoveryRuns),
		fmt.Sprintf("Last     %d bulbs / %dms", m.lastDiscoveryCount, m.lastDiscoveryMs),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.discoveryLatencyMs, 22)),
	}, blue, 34)

	return strings.Join([]string{core, brightnessBlock, commandBlock, discoveryBlock}, "\n")
}
