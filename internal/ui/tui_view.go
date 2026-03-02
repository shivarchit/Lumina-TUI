package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"wiz-tui/internal/version"
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

	borderColor := surface
	if m.isOn {
		borderColor = mauve
	}

	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(62).
		Height(24)

	rightPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(surface).
		Padding(1, 2).
		Width(44).
		Height(24)

	itemStyle := lipgloss.NewStyle().Foreground(subtext).PaddingLeft(1)
	selectedStyle := lipgloss.NewStyle().Foreground(mauve).Bold(true).PaddingLeft(1)

	var leftPanel string
	switch m.state {
	case menuView:
		leftPanel = sectionHeader("Control Board", "Main actions") + "\n\n"
		for i, choice := range m.choices {
			icon := m.icons[i]
			if m.cursor == i {
				leftPanel += selectedStyle.Render(fmt.Sprintf("┃ %s  %s", icon, choice)) + "\n"
			} else {
				leftPanel += itemStyle.Render(fmt.Sprintf("  %s  %s", icon, choice)) + "\n"
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
		leftPanel = sectionHeader("Hex Input", "Custom color") + "\n\n" + m.textInput.View()
	case brightnessView:
		leftPanel = sectionHeader("Brightness", "Fine control") + "\n\n"
		bars := m.brightness / 10
		slider := strings.Repeat("━", bars) + "┫" + strings.Repeat("┈", 10-bars)
		coloredSlider := lipgloss.NewStyle().Foreground(mauve).Render(slider)
		leftPanel += fmt.Sprintf("%s %d%%", coloredSlider, m.brightness)
	case timerInputView:
		leftPanel = sectionHeader("Sleep Timer", "Minutes") + "\n\n" + m.textInput.View()
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
			leftPanel += lipgloss.NewStyle().Foreground(textCol).Bold(true).Render("Name               IP              MAC\n")
			leftPanel += lipgloss.NewStyle().Foreground(surface).Render(strings.Repeat("─", 56)) + "\n"
			for i, device := range m.discoveredDevices {
				prefix := "  "
				style := lipgloss.NewStyle().Foreground(subtext)
				if i%2 == 1 {
					style = style.Foreground(textCol)
				}
				if i == m.deviceCursor {
					prefix = "▶ "
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
				}
				name := device.Name
				if len(name) > 16 {
					name = name[:16]
				}
				mac := device.Mac
				if mac == "" {
					mac = "-"
				}
				row := fmt.Sprintf("%s%-16s %-15s %s", prefix, name, device.IP, mac)
				leftPanel += style.Render(row) + "\n"
			}
			leftPanel += "\nEnter select · s save name · r refresh"
		}
	case savedDevicesView:
		leftPanel = sectionHeader("Saved Devices", "Persistent targets") + "\n\n"
		if len(m.savedDevices) == 0 {
			leftPanel += "No saved devices yet.\nDiscover a bulb and press 's' to save."
		} else {
			leftPanel += lipgloss.NewStyle().Foreground(textCol).Bold(true).Render("Name               IP              Port\n")
			leftPanel += lipgloss.NewStyle().Foreground(surface).Render(strings.Repeat("─", 56)) + "\n"
			for i, device := range m.savedDevices {
				prefix := "  "
				style := lipgloss.NewStyle().Foreground(subtext)
				if i%2 == 1 {
					style = style.Foreground(textCol)
				}
				if i == m.savedDeviceCursor {
					prefix = "▶ "
					style = lipgloss.NewStyle().Foreground(mauve).Bold(true)
				}
				name := device.Name
				if len(name) > 16 {
					name = name[:16]
				}
				port := device.Port
				if port == "" {
					port = m.port
				}
				row := fmt.Sprintf("%s%-16s %-15s %s", prefix, name, device.IP, port)
				leftPanel += style.Render(row) + "\n"
			}
			leftPanel += "\nEnter select · d delete · Esc back"
		}
	case saveDeviceNameView:
		leftPanel = sectionHeader("Save Device", "Enter display name") + "\n\n"
		leftPanel += m.textInput.View() + "\n\n"
		leftPanel += lipgloss.NewStyle().Foreground(subtext).Render("Enter to save · Esc to cancel")
	}

	rightPanel := m.renderDashboard()
	actionBlock := metricBlock("Action Feed", []string{m.status}, mauve, 34)
	rightPanel += "\n" + actionBlock

	if m.timerActive {
		rightPanel += fmt.Sprintf("\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(blue).Render("Timer Active"))
	}

	leftBox := leftPanelStyle.Render(leftPanel)
	rightBox := rightPanelStyle.Render(rightPanel)
	mainUI := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

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
