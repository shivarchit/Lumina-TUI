package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
	"github.com/shivarchit/Lumina-TUI/pkg/wiz"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	setupView sessionState = iota
	menuView
	colorPickerView
	hexInputView
	brightnessView
	colorTempView
	timerInputView
	discoveryView
	savedDevicesView
	saveDeviceNameView
	helpView
	themesView
	scenesView
	groupsView
	groupNameView
)

type timerFinishedMsg struct{}

type discoveryResultMsg struct {
	devices []wiz.Device
	err     error
	elapsed time.Duration
}

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

type macResolutionResultMsg struct {
	device wiz.Device
	err    error
}

type commandResultMsg struct {
	err        error
	elapsed    time.Duration
	onSuccess  func(*model)
	successMsg string
	failPrefix string
}

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

var colorPalette = []struct{ name, hex string }{
	{"Warm", "#FFB56B"}, {"Day", "#FFE4CE"}, {"Cool", "#E0F7FA"},
	{"Ruby", "#FF0033"}, {"Rose", "#FF66CC"}, {"Pink", "#FFB6C1"},
	{"Peach", "#FF9966"}, {"Orng", "#FF8C00"}, {"Gold", "#FFD700"},
	{"Lime", "#32CD32"}, {"Mint", "#98FF98"}, {"Emrld", "#00FF00"},
	{"Teal", "#008080"}, {"Aqua", "#00FFFF"}, {"Sky", "#87CEEB"},
	{"Ocean", "#006994"}, {"Blue", "#0000FF"}, {"Navy", "#000080"},
	{"Lvndr", "#E6E6FA"}, {"Prple", "#800080"}, {"Mgnta", "#FF00FF"},
}

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

type model struct {
	state         sessionState
	setupStep     int
	choices       []string
	icons         []string
	cursor        int
	colorCursor   int
	sceneCursor   int
	status        string
	ip, port      string
	isOn          bool
	currentColor  string
	brightness    int
	textInput     textinput.Model
	spinner       spinner.Model
	timerActive   bool
	detachedTimer bool
	syncingState  bool

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
	activeMac          string
	statusLog          []string
	statusLevel        statusLevel
	lastKeyWasG        bool
	colorTemp          int
	windowWidth        int
	windowHeight       int
	themeName          string
	themeCursor        int
	lastScene          int

	groups       []config.Group
	groupCursor  int
	memberCursor int
	editingGroup int
	activeGroup  string

	lastSyncAt time.Time
	lastSyncOK bool
	whiteMode  bool
}

// NewModel creates the first TUI model from runtime config.
func NewModel(cfg config.Config, needsSetup bool) model {
	themeName := applyTheme(cfg.Theme)

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

	initColor := "#CBA6F7"
	if cfg.LastColor != "" {
		initColor = cfg.LastColor
	}
	initBrightness := 100
	if cfg.LastBrightness > 0 {
		initBrightness = cfg.LastBrightness
	}
	initTemp := 4000
	if cfg.LastColorTemp > 0 {
		initTemp = cfg.LastColorTemp
	}

	themeCursor := 0
	for i, name := range themeOrder {
		if name == themeName {
			themeCursor = i
			break
		}
	}

	return model{
		state:              state,
		setupStep:          0,
		choices:            []string{"Toggle Power", "Scenes", "Groups", "Color Grid", "Hex Colors", "Brightness", "Color Temp", "Sleep Timer", "Discover Devices", "Saved Devices", "Theme", "Help", "Exit"},
		icons:              []string{"PWR", "SCN", "GRP", "CLR", "HEX", "BRT", "CCT", "TMR", "DSC", "SAV", "THM", "HLP", "EXT"},
		themeName:          themeName,
		themeCursor:        themeCursor,
		status:             "Ready.",
		statusLog:          []string{"Ready."},
		ip:                 cfg.IP,
		port:               cfg.Port,
		activeMac:          activeMac(cfg),
		isOn:               true,
		currentColor:       initColor,
		brightness:         initBrightness,
		colorTemp:          initTemp,
		textInput:          ti,
		spinner:            s,
		discoveredDevices:  []wiz.Device{},
		deviceCursor:       0,
		savedDevices:       cfg.SavedDevices,
		syncingState:       !needsSetup && strings.TrimSpace(cfg.IP) != "" && strings.TrimSpace(cfg.Port) != "",
		brightnessHistory:  []int{initBrightness},
		commandLatencyMs:   []int{},
		discoveryLatencyMs: []int{},
		windowWidth:        120,
		windowHeight:       36,
		lastScene:          cfg.LastScene,
		groups:             cfg.Groups,
		editingGroup:       -1,
	}
}

// activeMac returns the MAC for the saved device matching the current target IP.
func activeMac(cfg config.Config) string {
	for _, saved := range cfg.SavedDevices {
		if saved.IP == cfg.IP && strings.TrimSpace(saved.Mac) != "" {
			return strings.ToLower(strings.TrimSpace(saved.Mac))
		}
	}
	return ""
}

// persistConfig saves current target, saved devices, and last known state to disk.
func (m *model) persistConfig() {
	_ = config.Save(config.Config{
		Version:        1,
		IP:             m.ip,
		Port:           m.port,
		SavedDevices:   m.savedDevices,
		LastColor:      m.currentColor,
		LastBrightness: m.brightness,
		LastColorTemp:  m.colorTemp,
		Theme:          m.themeName,
		LastScene:      m.lastScene,
		Groups:         m.groups,
	})
}

// adjustBrightnessCmd steps brightness by delta (clamped 1-100), updates the
// model immediately so rapid keypresses accumulate, and returns the async send.
func (m *model) adjustBrightnessCmd(delta int) tea.Cmd {
	newVal := m.brightness + delta
	if newVal > 100 {
		newVal = 100
	}
	if newVal < 1 {
		newVal = 1
	}
	m.brightness = newVal
	m.brightnessHistory = appendBounded(m.brightnessHistory, newVal, 30)

	return m.sendToTarget("setPilot", map[string]interface{}{"dimming": newVal},
		fmt.Sprintf("Bright: %d%%", newVal), "Brightness change failed",
		func(mm *model) {
			mm.isOn = true
			mm.persistConfig()
		})
}

// normalizeHex canonicalizes user hex input to "#RRGGBB" uppercase.
func normalizeHex(val string) string {
	return "#" + strings.ToUpper(strings.TrimPrefix(val, "#"))
}

type statusLevel int

const (
	statusInfo statusLevel = iota
	statusSuccess
	statusError
)

// pushStatus updates the current status and appends it to the bounded history log.
func pushStatus(m model, level statusLevel, s string) model {
	m.status = s
	m.statusLevel = level
	m.statusLog = appendBoundedStr(m.statusLog, s, 8)
	return m
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

// syncDeviceStateCmd fetches current target state asynchronously.
func syncDeviceStateCmd(ip, port string, quiet bool) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		state, err := wiz.GetPilotState(ip, port)
		return stateSyncResultMsg{state: state, err: err, elapsed: time.Since(start), quiet: quiet}
	}
}

// resolveDeviceCmd broadcasts and returns as soon as the target MAC responds.
// On success it chains into a state sync automatically via the Update loop.
func resolveDeviceCmd(mac, port string) tea.Cmd {
	return func() tea.Msg {
		device, err := wiz.DiscoverDeviceByMAC(mac, port, 1500*time.Millisecond)
		return macResolutionResultMsg{device: device, err: err}
	}
}

// sendCmd sends a device command asynchronously. Empty successMsg and
// failPrefix mark a silent send: no status push, no telemetry.
func sendCmd(ip, port, method string, params map[string]interface{},
	successMsg, failPrefix string, onSuccess func(*model)) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		err := wiz.SendCommand(ip, port, method, params)
		return commandResultMsg{
			err:        err,
			elapsed:    time.Since(start),
			onSuccess:  onSuccess,
			successMsg: successMsg,
			failPrefix: failPrefix,
		}
	}
}

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

// appendBoundedStr appends to a string history slice and keeps it capped.
func appendBoundedStr(history []string, value string, maxLen int) []string {
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

	latencyColor := green
	if latestCmdLatency >= 250 {
		latencyColor = red
	} else if latestCmdLatency >= 120 {
		latencyColor = mauve
	}

	targetAlias := m.currentTargetSavedName()
	aliasLine := "Alias    -"
	if strings.TrimSpace(targetAlias) != "" {
		aliasLine = fmt.Sprintf("Alias    %s", targetAlias)
	}

	colorSwatch := lipgloss.NewStyle().
		Background(lipgloss.Color(m.currentColor)).
		Foreground(base).
		Padding(0, 2).
		Render("  ")

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

	targetLine := fmt.Sprintf("Target   %s:%s", m.ip, m.port)
	if m.activeGroup != "" {
		targetLine = fmt.Sprintf("Target   group: %s", m.activeGroup)
	}

	core := metricBlock("Core", []string{
		fmt.Sprintf("Power    %s", powerStyle.Bold(true).Render(powerState)),
		targetLine,
		aliasLine,
		modeLine,
		syncLine,
	}, blue, 34)

	brightnessBlock := metricBlock("Brightness", []string{
		lipgloss.NewStyle().Foreground(mauve).Render(bar(m.brightness, 100, 22)),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.brightnessHistory, 22)),
		fmt.Sprintf("Level    %d%%", m.brightness),
	}, mauve, 34)

	commandBlock := metricBlock("Command Health", []string{
		lipgloss.NewStyle().Foreground(green).Render(bar(successRate, 100, 22)),
		fmt.Sprintf("OK/Fail  %d/%d", successCount, m.commandFailed),
		fmt.Sprintf("Latency  %s", lipgloss.NewStyle().Foreground(latencyColor).Bold(true).Render(fmt.Sprintf("%dms", latestCmdLatency))),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.commandLatencyMs, 22)),
	}, green, 34)

	discoveryBlock := metricBlock("Discovery", []string{
		fmt.Sprintf("Runs     %d", m.discoveryRuns),
		fmt.Sprintf("Last     %d bulbs / %dms", m.lastDiscoveryCount, m.lastDiscoveryMs),
		lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.discoveryLatencyMs, 22)),
	}, blue, 34)

	return strings.Join([]string{core, brightnessBlock, commandBlock, discoveryBlock}, "\n")
}
