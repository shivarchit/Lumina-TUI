package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"wiz-tui/internal/config"
	"wiz-tui/internal/wiz"
)

// Init configures startup commands for text input and spinner.
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// Update handles all messages and user interactions for the TUI model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.timerActive || m.discovering {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	case timerFinishedMsg:
		m.timerActive = false
		m.isOn = false
		if m.detachedTimer {
			m.status = "Timer finished (handled in background)"
		} else {
			start := time.Now()
			err := wiz.SendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": false})
			m.recordCommand(time.Since(start), err)
			if err != nil {
				m.status = fmt.Sprintf("Timer finished. Power off failed: %v", err)
			} else {
				m.status = "Timer finished. Power off."
			}
		}
		return m, nil
	case discoveryResultMsg:
		m.discovering = false
		m.discoveryRuns++
		m.lastDiscoveryMs = int(msg.elapsed.Milliseconds())
		m.discoveryLatencyMs = appendBounded(m.discoveryLatencyMs, m.lastDiscoveryMs, 30)
		if msg.err != nil {
			m.status = fmt.Sprintf("Discovery failed: %v", msg.err)
			return m, nil
		}
		m.discoveredDevices = msg.devices
		m.applySavedNamesToDiscovered()
		m.lastDiscoveryCount = len(msg.devices)
		if len(m.discoveredDevices) == 0 {
			m.deviceCursor = 0
			m.status = "Discovery complete: no bulbs found"
		} else {
			if m.deviceCursor >= len(m.discoveredDevices) {
				m.deviceCursor = len(m.discoveredDevices) - 1
			}
			m.status = fmt.Sprintf("Discovery complete: %d bulb(s)", len(m.discoveredDevices))
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.state == setupView {
			switch msg.String() {
			case "enter":
				if m.setupStep == 0 {
					m.ip = m.textInput.Value()
					if m.ip == "" {
						m.ip = "192.168.1.2"
					}
					m.setupStep = 1
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. 38899"
				} else {
					m.port = m.textInput.Value()
					if m.port == "" {
						m.port = "38899"
					}

					m.persistConfig()

					m.state = menuView
					m.textInput.Blur()
					m.textInput.SetValue("")
					m.status = "Config saved"
				}
			case "esc":
				return m, tea.Quit
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

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
					start := time.Now()
					err := wiz.SendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": m.isOn})
					m.recordCommand(time.Since(start), err)
					if err != nil {
						m.status = fmt.Sprintf("Power toggle failed: %v", err)
						m.isOn = !m.isOn
					} else if m.isOn {
						m.status = "Power: ON"
					} else {
						m.status = "Power: OFF"
					}
				case 1:
					m.state = colorPickerView
				case 2:
					m.state = hexInputView
					m.textInput.CharLimit = 7
					m.textInput.Placeholder = "#CBA6F7"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 3:
					m.state = brightnessView
				case 4:
					m.state = timerInputView
					m.textInput.CharLimit = 5
					m.textInput.Placeholder = "Mins (e.g. 15)"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 5:
					m.state = discoveryView
					m.discovering = true
					m.status = "Scanning local network..."
					cmds = append(cmds, discoverDevicesCmd(), m.spinner.Tick)
				case 6:
					m.state = savedDevicesView
				case 7:
					m.state = helpView
				case 8:
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
				r, g, b, _ := wiz.HexToRGB(selectedHex)
				start := time.Now()
				err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				m.recordCommand(time.Since(start), err)
				if err != nil {
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
				r, g, b, err := wiz.HexToRGB(val)
				if err != nil {
					m.status = "Err: Invalid Hex"
				} else {
					start := time.Now()
					cmdErr := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
					m.recordCommand(time.Since(start), cmdErr)
					if cmdErr != nil {
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
					start := time.Now()
					err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness})
					m.recordCommand(time.Since(start), err)
					if err != nil {
						m.status = fmt.Sprintf("Brightness change failed: %v", err)
						m.brightness += 10
					} else {
						m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
						m.brightnessHistory = appendBounded(m.brightnessHistory, m.brightness, 30)
					}
				}
			case "right", "l":
				if m.brightness < 100 {
					m.brightness += 10
					start := time.Now()
					err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": m.brightness})
					m.recordCommand(time.Since(start), err)
					if err != nil {
						m.status = fmt.Sprintf("Brightness change failed: %v", err)
						m.brightness -= 10
					} else {
						m.status = fmt.Sprintf("Bright: %d%%", m.brightness)
						m.brightnessHistory = appendBounded(m.brightnessHistory, m.brightness, 30)
					}
				}
			}
		case timerInputView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "enter":
				val := m.textInput.Value()
				mins, err := strconv.Atoi(val)
				if err == nil && mins > 0 {
					m.timerActive = true
					m.detachedTimer = false
					m.status = fmt.Sprintf("Sleep in %dm", mins)
					cmds = append(cmds, startTimer(time.Duration(mins)*time.Minute), m.spinner.Tick)
					if spawnErr := startDetachedTimer(mins, m.ip, m.port); spawnErr == nil {
						m.detachedTimer = true
						m.status = fmt.Sprintf("Sleep in %dm (background armed)", mins)
					} else {
						m.status = fmt.Sprintf("Sleep in %dm (local only): %v", mins, spawnErr)
					}
				} else {
					m.status = "Invalid timer value"
				}
				m.state = menuView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		case discoveryView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "r":
				if !m.discovering {
					m.discovering = true
					m.status = "Rescanning local network..."
					cmds = append(cmds, discoverDevicesCmd(), m.spinner.Tick)
				}
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
					m.persistConfig()
					m.status = fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.IP)
					m.state = menuView
				}
			case "s":
				if len(m.discoveredDevices) > 0 {
					m.pendingSaveDevice = m.discoveredDevices[m.deviceCursor]
					m.textInput.CharLimit = 32
					m.textInput.Placeholder = "Saved name"
					m.textInput.SetValue(m.pendingSaveDevice.Name)
					m.textInput.Focus()
					m.state = saveDeviceNameView
				}
			}
		case savedDevicesView:
			switch msg.String() {
			case "esc", "q":
				m.state = menuView
			case "up", "k":
				if m.savedDeviceCursor > 0 {
					m.savedDeviceCursor--
				}
			case "down", "j":
				if m.savedDeviceCursor < len(m.savedDevices)-1 {
					m.savedDeviceCursor++
				}
			case "enter":
				if len(m.savedDevices) > 0 {
					selected := m.savedDevices[m.savedDeviceCursor]
					m.ip = selected.IP
					if selected.Port != "" {
						m.port = selected.Port
					}
					m.persistConfig()
					m.status = fmt.Sprintf("Selected saved device: %s", selected.Name)
					m.state = menuView
				}
			case "d":
				if len(m.savedDevices) > 0 {
					name := m.savedDevices[m.savedDeviceCursor].Name
					m.deleteSavedDevice()
					m.persistConfig()
					m.status = fmt.Sprintf("Removed saved device: %s", name)
				}
			}
		case saveDeviceNameView:
			switch msg.String() {
			case "esc":
				m.textInput.Blur()
				m.state = discoveryView
			case "enter":
				if strings.TrimSpace(m.pendingSaveDevice.Mac) == "" {
					m.status = "Cannot save device without MAC"
					m.state = discoveryView
					m.textInput.Blur()
					break
				}

				name := strings.TrimSpace(m.textInput.Value())
				if name == "" {
					name = m.pendingSaveDevice.Name
				}
				if name == "" {
					name = "WiZ Device"
				}

				saved := config.SavedDevice{
					Name: name,
					IP:   m.pendingSaveDevice.IP,
					Port: m.port,
					Mac:  m.pendingSaveDevice.Mac,
				}
				m.upsertSavedDevice(saved)
				m.applySavedNamesToDiscovered()
				m.ip = saved.IP
				m.persistConfig()
				m.textInput.Blur()
				m.status = fmt.Sprintf("Saved device: %s", saved.Name)
				m.state = discoveryView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		case helpView:
			switch msg.String() {
			case "esc", "q", "enter":
				m.state = menuView
			}
		}
	}
	return m, tea.Batch(cmds...)
}
