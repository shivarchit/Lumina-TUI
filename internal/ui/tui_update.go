package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"wiz-tui/internal/config"
	"wiz-tui/internal/wiz"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Init configures startup commands for text input and spinner.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.spinner.Tick}
	if m.state != setupView && m.activeMac != "" {
		// Resolve IP via MAC in the background for instant boot
		m.syncingState = true
		cmds = append(cmds, resolveDeviceCmd(m.activeMac, m.port), m.spinner.Tick)
	} else if m.state != setupView && m.ip != "" && m.port != "" {
		cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port))
	}
	return tea.Batch(cmds...)
}

// Update handles all messages and user interactions for the TUI model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.windowWidth = msg.Width
		}
		if msg.Height > 0 {
			m.windowHeight = msg.Height
		}
		return m, nil
	case spinner.TickMsg:
		if m.timerActive || m.discovering || m.syncingState {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	case timerFinishedMsg:
		m.timerActive = false
		m.isOn = false
		if m.detachedTimer {
			m = pushStatus(m, "Timer finished (handled in background)")
		} else {
			start := time.Now()
			err := wiz.SendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": false})
			m.recordCommand(time.Since(start), err)
			if err != nil {
				m = pushStatus(m, fmt.Sprintf("Timer finished. Power off failed: %v", err))
			} else {
				m = pushStatus(m, "Timer finished. Power off.")
			}
		}
		return m, nil
	case discoveryResultMsg:
		m.discovering = false
		m.discoveryRuns++
		m.lastDiscoveryMs = int(msg.elapsed.Milliseconds())
		m.discoveryLatencyMs = appendBounded(m.discoveryLatencyMs, m.lastDiscoveryMs, 30)
		if msg.err != nil {
			m = pushStatus(m, fmt.Sprintf("Discovery failed: %v", msg.err))
			return m, nil
		}
		m.discoveredDevices = msg.devices
		m.applySavedNamesToDiscovered()
		m.lastDiscoveryCount = len(msg.devices)
		if len(m.discoveredDevices) == 0 {
			m.deviceCursor = 0
			m = pushStatus(m, "Discovery complete: no bulbs found")
		} else {
			if m.deviceCursor >= len(m.discoveredDevices) {
				m.deviceCursor = len(m.discoveredDevices) - 1
			}
			m = pushStatus(m, fmt.Sprintf("Discovery complete: %d bulb(s)", len(m.discoveredDevices)))
		}
	case stateSyncResultMsg:
		m.syncingState = false
		m.commandLatencyMs = appendBounded(m.commandLatencyMs, int(msg.elapsed.Milliseconds()), 30)
		if msg.err != nil {
			m = pushStatus(m, fmt.Sprintf("State sync failed: %v", msg.err))
			return m, nil
		}
		m.isOn = msg.state.Power
		if msg.state.Brightness > 0 {
			m.brightness = msg.state.Brightness
			m.brightnessHistory = appendBounded(m.brightnessHistory, m.brightness, 30)
		}
		if strings.TrimSpace(msg.state.ColorHex) != "" {
			m.currentColor = msg.state.ColorHex
		}
		m = pushStatus(m, "State synced")
		return m, nil
	case macResolutionResultMsg:
		if msg.err != nil {
			m.syncingState = false
			if m.ip != "" && m.port != "" {
				m.syncingState = true
				return m, syncDeviceStateCmd(m.ip, m.port)
			}
			m = pushStatus(m, fmt.Sprintf("Device not found on network: %v", msg.err))
			return m, nil
		}
		m.ip = msg.device.IP
		m.activeMac = msg.device.Mac
		for i := range m.savedDevices {
			if strings.ToLower(strings.TrimSpace(m.savedDevices[i].Mac)) == m.activeMac {
				m.savedDevices[i].IP = m.ip
			}
		}
		m.persistConfig()
		m.syncingState = true
		return m, tea.Batch(syncDeviceStateCmd(m.ip, m.port), m.spinner.Tick)
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
					m = pushStatus(m, "Config saved")
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
				m.lastKeyWasG = false
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				m.lastKeyWasG = false
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			case "g":
				if m.lastKeyWasG {
					m.cursor = 0
					m.lastKeyWasG = false
				} else {
					m.lastKeyWasG = true
				}
			case "G":
				m.lastKeyWasG = false
				m.cursor = len(m.choices) - 1
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				m.lastKeyWasG = false
				idx, _ := strconv.Atoi(msg.String())
				if idx == 0 {
					idx = 10
				}
				idx-- // 1-indexed to 0-indexed
				if idx < len(m.choices) {
					m.cursor = idx
				}
			case "enter", " ":
				m.lastKeyWasG = false
				switch m.cursor {
				case 0: // Toggle Power
					m.isOn = !m.isOn
					start := time.Now()
					err := wiz.SendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": m.isOn})
					m.recordCommand(time.Since(start), err)
					if err != nil {
						m = pushStatus(m, fmt.Sprintf("Power toggle failed: %v", err))
						m.isOn = !m.isOn
					} else if m.isOn {
						m = pushStatus(m, "Power: ON")
					} else {
						m = pushStatus(m, "Power: OFF")
					}
				case 1: // Color Grid
					m.state = colorPickerView
				case 2: // Hex Colors
					m.state = hexInputView
					m.textInput.CharLimit = 7
					m.textInput.Placeholder = "#CBA6F7"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 3: // Brightness
					m.state = brightnessView
				case 4: // Color Temp
					m.state = colorTempView
				case 5: // Sleep Timer
					m.state = timerInputView
					m.textInput.CharLimit = 5
					m.textInput.Placeholder = "Mins (e.g. 15)"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 6: // Discover Devices
					m.state = discoveryView
					m.discovering = true
					m = pushStatus(m, "Scanning local network...")
					cmds = append(cmds, discoverDevicesCmd(), m.spinner.Tick)
				case 7: // Saved Devices
					m.state = savedDevicesView
				case 8: // Help
					m.state = helpView
				case 9: // Exit
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
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					_ = wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				}
			case "down", "j":
				if m.colorCursor < len(colorPalette)-3 {
					m.colorCursor += 3
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					_ = wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				}
			case "left", "h":
				if m.colorCursor > 0 {
					m.colorCursor--
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					_ = wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				}
			case "right", "l":
				if m.colorCursor < len(colorPalette)-1 {
					m.colorCursor++
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					_ = wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				}
			case "enter":
				selectedHex := colorPalette[m.colorCursor].hex
				r, g, b, _ := wiz.HexToRGB(selectedHex)
				start := time.Now()
				err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
				m.recordCommand(time.Since(start), err)
				if err != nil {
					m = pushStatus(m, fmt.Sprintf("Color change failed: %v", err))
				} else {
					m.currentColor = selectedHex
					m.isOn = true
					m = pushStatus(m, "Color: "+colorPalette[m.colorCursor].name)
					m.persistConfig()
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
					m = pushStatus(m, "Err: Invalid Hex")
				} else {
					start := time.Now()
					cmdErr := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness})
					m.recordCommand(time.Since(start), cmdErr)
					if cmdErr != nil {
						m = pushStatus(m, fmt.Sprintf("Color change failed: %v", cmdErr))
					} else {
						m.currentColor = val
						m.isOn = true
						m = pushStatus(m, fmt.Sprintf("Color: %s", val))
						m.persistConfig()
					}
				}
				m.state = menuView
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		case brightnessView:
			sendBrightness := func(newVal int) {
				start := time.Now()
				err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"dimming": newVal})
				m.recordCommand(time.Since(start), err)
				if err != nil {
					m = pushStatus(m, fmt.Sprintf("Brightness change failed: %v", err))
				} else {
					m.brightness = newVal
					m = pushStatus(m, fmt.Sprintf("Bright: %d%%", m.brightness))
					m.brightnessHistory = appendBounded(m.brightnessHistory, m.brightness, 30)
					m.persistConfig()
				}
			}
			switch msg.String() {
			case "esc", "q", "enter":
				m.state = menuView
			case "left", "h":
				if m.brightness > 10 {
					sendBrightness(m.brightness - 10)
				}
			case "right", "l":
				if m.brightness < 100 {
					sendBrightness(m.brightness + 10)
				}
			case "-", "_":
				if m.brightness > 1 {
					sendBrightness(m.brightness - 1)
				}
			case "+", "=":
				if m.brightness < 100 {
					sendBrightness(m.brightness + 1)
				}
			}
		case colorTempView:
			sendColorTemp := func(k int) {
				start := time.Now()
				err := wiz.SendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"temp": k, "dimming": m.brightness})
				m.recordCommand(time.Since(start), err)
				if err != nil {
					m = pushStatus(m, fmt.Sprintf("Color temp failed: %v", err))
				} else {
					m.colorTemp = k
					m = pushStatus(m, fmt.Sprintf("Temp: %dK", k))
				}
			}
			switch msg.String() {
			case "esc", "q", "enter":
				m.state = menuView
			case "left", "h":
				if m.colorTemp > 2200 {
					sendColorTemp(m.colorTemp - 100)
				}
			case "right", "l":
				if m.colorTemp < 6500 {
					sendColorTemp(m.colorTemp + 100)
				}
			case "-":
				if m.colorTemp > 2200 {
					sendColorTemp(m.colorTemp - 10)
				}
			case "+", "=":
				if m.colorTemp < 6500 {
					sendColorTemp(m.colorTemp + 10)
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
					m = pushStatus(m, fmt.Sprintf("Sleep in %dm", mins))
					cmds = append(cmds, startTimer(time.Duration(mins)*time.Minute), m.spinner.Tick)
					if spawnErr := startDetachedTimer(mins, m.ip, m.port); spawnErr == nil {
						m.detachedTimer = true
						m = pushStatus(m, fmt.Sprintf("Sleep in %dm (background armed)", mins))
					} else {
						m = pushStatus(m, fmt.Sprintf("Sleep in %dm (local only): %v", mins, spawnErr))
					}
				} else {
					m = pushStatus(m, "Invalid timer value")
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
					m = pushStatus(m, "Rescanning local network...")
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
					m.activeMac = strings.ToLower(strings.TrimSpace(selectedDevice.Mac))
					m.persistConfig()
					m = pushStatus(m, fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.IP))
					m.state = menuView
					m.syncingState = true
					cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port), m.spinner.Tick)
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
					m.activeMac = strings.ToLower(strings.TrimSpace(selected.Mac))
					m.persistConfig()
					m = pushStatus(m, fmt.Sprintf("Selected saved device: %s", selected.Name))
					m.state = menuView
					if m.activeMac != "" {
						// Fast MAC-based resolution in background
						m.syncingState = true
						cmds = append(cmds, resolveDeviceCmd(m.activeMac, m.port), m.spinner.Tick)
					} else {
						m.syncingState = true
						cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port), m.spinner.Tick)
					}
				}
			case "d":
				if len(m.savedDevices) > 0 {
					name := m.savedDevices[m.savedDeviceCursor].Name
					m.deleteSavedDevice()
					m.persistConfig()
					m = pushStatus(m, fmt.Sprintf("Removed saved device: %s", name))
				}
			}
		case saveDeviceNameView:
			switch msg.String() {
			case "esc":
				m.textInput.Blur()
				m.state = discoveryView
			case "enter":
				if strings.TrimSpace(m.pendingSaveDevice.Mac) == "" {
					m = pushStatus(m, "Cannot save device without MAC")
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
				m = pushStatus(m, fmt.Sprintf("Saved device: %s", saved.Name))
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
