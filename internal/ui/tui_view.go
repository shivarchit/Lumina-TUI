package ui

import (
	"fmt"
	"strings"

	"wiz-tui/internal/version"

	"github.com/charmbracelet/lipgloss"
)

// View renders the complete application UI for the current model state.
func (m model) View() string {
	if m.state == setupView {
		prompt := "Enter WiZ Device IP Address:"
		if m.setupStep == 1 {
			prompt = "Enter UDP Port (Default 38899):"
		}

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mauve).
			Padding(2, 4).
			Width(48).
			Align(lipgloss.Center)

		content := fmt.Sprintf("%s\n\n%s\n\n%s",
			lipgloss.NewStyle().Bold(true).Foreground(mauve).Render("FIRST-TIME SETUP"),
			prompt,
			m.textInput.View())

		return "\n" + box.Render(content) + "\n"
	}

	narrow := m.windowWidth > 0 && m.windowWidth < 118
	leftWidth := 62
	rightWidth := 44
	panelHeight := 24
	cardWidth := 54
	if narrow {
		leftWidth = maxInt(52, m.windowWidth-8)
		rightWidth = leftWidth
		panelHeight = 20
	}
	if leftWidth > 10 {
		cardWidth = leftWidth - 8
	}

	activeBorder := mauve
	inactiveBorder := lipgloss.Color("#45475A")
	if !m.isOn {
		activeBorder = blue
	}

	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeBorder).
		Padding(1, 2).
		Width(leftWidth).
		Height(panelHeight)

	rightPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inactiveBorder).
		Padding(1, 2).
		Width(rightWidth).
		Height(panelHeight)

	itemStyle := lipgloss.NewStyle().Foreground(subtext).PaddingLeft(1)
	selectedStyle := lipgloss.NewStyle().Foreground(mauve).Bold(true).PaddingLeft(1)

	var leftPanel string
	switch m.state {
	case menuView:
		leftPanel = sectionHeader("Control Board", "Main actions") + "\n\n"
		for i, choice := range m.choices {
			icon := m.icons[i]
			if m.cursor == i {
				leftPanel += selectedStyle.Render(fmt.Sprintf("> %-3s %s", icon, choice)) + "\n"
			} else {
				leftPanel += itemStyle.Render(fmt.Sprintf("  %-3s %s", icon, choice)) + "\n"
			}
		}
		leftPanel += "\n"
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Tip: open Discover Devices to auto-start network scan")
	case colorPickerView:
		leftPanel = sectionHeader("Color Matrix", "Preset palette") + "\n\n"
		for i, c := range colorPalette {
			text := c.name
			bg := lipgloss.Color(c.hex)
			fg := lipgloss.Color("#11111B")
			if m.colorCursor == i {
				text = "> " + text
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
		swatch := lipgloss.NewStyle().Background(lipgloss.Color(m.currentColor)).Foreground(base).Padding(0, 3).Render("   ")
		leftPanel = sectionHeader("Hex Input", "Custom color") + "\n\n"
		leftPanel += "Current " + swatch + " " + lipgloss.NewStyle().Foreground(mauve).Render(m.currentColor) + "\n\n"
		leftPanel += m.textInput.View() + "\n\n"
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Enter to apply · Esc to cancel")
	case brightnessView:
		leftPanel = sectionHeader("Brightness", "Fine control") + "\n\n"
		leftPanel += lipgloss.NewStyle().Foreground(mauve).Render(bar(m.brightness, 100, 28)) + "\n"
		leftPanel += lipgloss.NewStyle().Foreground(blue).Render(sparkline(m.brightnessHistory, 28)) + "\n"
		leftPanel += fmt.Sprintf("Level  %d%%\n\n", m.brightness)
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Left/Right to adjust · Enter/Esc to return")
	case timerInputView:
		leftPanel = sectionHeader("Sleep Timer", "Minutes") + "\n\n"
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Set minutes until automatic power off") + "\n\n"
		leftPanel += m.textInput.View() + "\n\n"
		if m.timerActive {
			leftPanel += lipgloss.NewStyle().Foreground(blue).Render("Timer running in background")
		}
	case helpView:
		leftPanel = sectionHeader("Help", "Key reference") + "\n\n" +
			lipgloss.NewStyle().Foreground(textCol).Render("Navigation:\n") +
			"↑↓/jk   Move cursor\n" +
			"Enter    Select/Confirm\n" +
			"Esc      Cancel/Back\n" +
			"q/Ctrl+C Quit\n" +
			"r        Refresh discovery\n" +
			"s        Save discovered device\n" +
			"d        Delete saved device\n\n" +
			lipgloss.NewStyle().Foreground(textCol).Render("Discovery:\n") +
			"Auto scan on open\n" +
			"Dedupe by MAC/IP\n" +
			"Save discovered bulbs with custom names"
	case discoveryView:
		subtitle := "Network scan"
		if m.discovering {
			subtitle = "Scanning in progress"
		}
		leftPanel = sectionHeader("Device Discovery", subtitle) + "\n\n"
		if m.discovering {
			leftPanel += fmt.Sprintf("%s Scanning...\n\n", m.spinner.View())
		}
		if len(m.discoveredDevices) == 0 {
			leftPanel += "No bulbs found yet.\nPress 'r' to rescan."
		} else {
			leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Choose a device and press Enter") + "\n\n"
			for i, device := range m.discoveredDevices {
				style := lipgloss.NewStyle().Foreground(textCol)
				if i == m.deviceCursor {
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
				}
				name := clipText(device.Name, 20)
				mac := device.Mac
				if mac == "" {
					mac = "-"
				}
				stateLabel := "unknown"
				if device.IP == m.ip {
					stateLabel = "active"
				}
				leftPanel += renderDeviceCard(name, device.IP, mac, stateLabel, style, i == m.deviceCursor, cardWidth) + "\n"
			}
			leftPanel += "\nEnter select · s save name · r refresh"
		}
	case savedDevicesView:
		leftPanel = sectionHeader("Saved Devices", "Persistent targets") + "\n\n"
		if len(m.savedDevices) == 0 {
			leftPanel += "No saved devices yet.\nDiscover a bulb and press 's' to save."
		} else {
			leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Enter to target · d to delete") + "\n\n"
			for i, device := range m.savedDevices {
				style := lipgloss.NewStyle().Foreground(textCol)
				if i == m.savedDeviceCursor {
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
				}
				name := clipText(device.Name, 20)
				port := device.Port
				if port == "" {
					port = m.port
				}
				mac := device.Mac
				if mac == "" {
					mac = "-"
				}
				leftPanel += renderDeviceCard(name, device.IP+":"+port, mac, "saved", style, i == m.savedDeviceCursor, cardWidth) + "\n"
			}
			leftPanel += "\nEnter select · d delete · Esc back"
		}
	case saveDeviceNameView:
		leftPanel = sectionHeader("Save Device", "Enter display name") + "\n\n"
		leftPanel += m.textInput.View() + "\n\n"
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Enter to save · Esc to cancel")
	}

	rightPanel := m.renderDashboard()
	actionBlock := renderStatusBlock(m.status, rightWidth-10)
	rightPanel += "\n" + actionBlock

	if m.timerActive {
		rightPanel += fmt.Sprintf("\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(blue).Render("Timer Active"))
	}

	leftBox := leftPanelStyle.Render(strings.TrimSpace(leftPanel))
	rightBox := rightPanelStyle.Render(strings.TrimSpace(rightPanel))
	mainUI := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	if narrow {
		mainUI = lipgloss.JoinVertical(lipgloss.Left, leftBox, rightBox)
	}

	modeStr := " NORMAL "
	modeBg := blue
	if m.state != menuView {
		modeStr = " INSERT "
		modeBg = mauve
	}

	modeBadge := lipgloss.NewStyle().Background(modeBg).Foreground(base).Bold(true).Render(modeStr)
	infoBadge := lipgloss.NewStyle().Background(surface).Foreground(textCol).Padding(0, 1).Render("Lumina")
	deviceBadge := lipgloss.NewStyle().Background(surface).Foreground(blue).Padding(0, 1).Render(fmt.Sprintf("Bulbs %d", len(m.discoveredDevices)))
	successRate := 100
	if m.commandTotal > 0 {
		successRate = ((m.commandTotal - m.commandFailed) * 100) / m.commandTotal
	}
	healthBadge := lipgloss.NewStyle().Background(base).Foreground(green).Padding(0, 1).Render(fmt.Sprintf("OK %d%%", successRate))
	versionBadge := lipgloss.NewStyle().Background(base).Foreground(subtext).Padding(0, 1).Render(version.Version)

	statusBar := lipgloss.JoinHorizontal(lipgloss.Top, modeBadge, infoBadge, deviceBadge, healthBadge, versionBadge)
	return "\n" + mainUI + "\n" + statusBar + "\n"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clipText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return value[:limit-1] + "…"
}

func renderDeviceCard(name, endpoint, mac, stateLabel string, style lipgloss.Style, selected bool, width int) string {
	border := surface
	if selected {
		border = mauve
	}
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(width)

	stateColor := subtext
	if stateLabel == "active" {
		stateColor = green
	}
	if stateLabel == "saved" {
		stateColor = blue
	}
	state := lipgloss.NewStyle().Foreground(stateColor).Bold(true).Render(strings.ToUpper(stateLabel))

	body := style.Render(name) + "  " + state + "\n" +
		lipgloss.NewStyle().Foreground(subtext).Render(endpoint) + "\n" +
		lipgloss.NewStyle().Foreground(subtext).Render("MAC "+mac)

	return card.Render(body)
}

func renderStatusBlock(status string, width int) string {
	label := "Info"
	accent := blue
	textStyle := lipgloss.NewStyle().Foreground(textCol)
	lower := strings.ToLower(status)

	switch {
	case strings.Contains(lower, "fail"), strings.Contains(lower, "error"), strings.Contains(lower, "invalid"):
		label = "Error"
		accent = red
		textStyle = lipgloss.NewStyle().Foreground(red)
	case strings.Contains(lower, "saved"), strings.Contains(lower, "complete"), strings.Contains(lower, "synced"), strings.Contains(lower, "on"):
		label = "Success"
		accent = green
		textStyle = lipgloss.NewStyle().Foreground(green)
	case strings.Contains(lower, "scan"), strings.Contains(lower, "timer"), strings.Contains(lower, "selected"):
		label = "Info"
		accent = blue
	}

	badge := lipgloss.NewStyle().Background(accent).Foreground(base).Bold(true).Padding(0, 1).Render(label)
	line := badge + " " + textStyle.Render(status)
	return metricBlock("Action Feed", []string{line}, accent, width)
}
