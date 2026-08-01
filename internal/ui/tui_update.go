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
		cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port, false))
	}
	cmds = append(cmds, pollCmd())
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
		if m.detachedTimer {
			m.isOn = false
			m = pushStatus(m, statusInfo, "Timer finished (handled in background)")
			return m, nil
		}
		return m, sendCmd(m.ip, m.port, "setState", map[string]interface{}{"state": false},
			"Timer finished. Power off.", "Timer finished. Power off failed",
			func(mm *model) { mm.isOn = false })
	case discoveryResultMsg:
		m.discovering = false
		m.discoveryRuns++
		m.lastDiscoveryMs = int(msg.elapsed.Milliseconds())
		m.discoveryLatencyMs = appendBounded(m.discoveryLatencyMs, m.lastDiscoveryMs, 30)
		if msg.err != nil {
			m = pushStatus(m, statusError, fmt.Sprintf("Discovery failed: %v", msg.err))
			return m, nil
		}
		m.discoveredDevices = msg.devices
		m.applySavedNamesToDiscovered()
		m.lastDiscoveryCount = len(msg.devices)
		if len(m.discoveredDevices) == 0 {
			m.deviceCursor = 0
			m = pushStatus(m, statusInfo, "Discovery complete: no bulbs found")
		} else {
			if m.deviceCursor >= len(m.discoveredDevices) {
				m.deviceCursor = len(m.discoveredDevices) - 1
			}
			m = pushStatus(m, statusSuccess, fmt.Sprintf("Discovery complete: %d bulb(s)", len(m.discoveredDevices)))
		}
	case pollTickMsg:
		cmds = append(cmds, pollCmd()) // always reschedule
		if m.state == menuView && m.ip != "" && m.port != "" {
			cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port, true))
		}
		return m, tea.Batch(cmds...)
	case stateSyncResultMsg:
		m.syncingState = false
		m.commandLatencyMs = appendBounded(m.commandLatencyMs, int(msg.elapsed.Milliseconds()), 30)
		m.lastSyncAt = time.Now()
		m.lastSyncOK = msg.err == nil
		if msg.err != nil {
			if msg.quiet {
				return m, nil
			}
			m = pushStatus(m, statusError, fmt.Sprintf("State sync failed: %v", msg.err))
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
		if msg.state.Temp > 0 {
			m.colorTemp = msg.state.Temp
		}
		m.whiteMode = msg.state.Temp > 0 && strings.TrimSpace(msg.state.ColorHex) == ""
		if !msg.quiet {
			m = pushStatus(m, statusSuccess, "State synced")
		}
		return m, nil
	case macResolutionResultMsg:
		if msg.err != nil {
			m.syncingState = false
			if m.ip != "" && m.port != "" {
				m.syncingState = true
				return m, syncDeviceStateCmd(m.ip, m.port, false)
			}
			m = pushStatus(m, statusError, fmt.Sprintf("Device not found on network: %v", msg.err))
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
		return m, tea.Batch(syncDeviceStateCmd(m.ip, m.port, false), m.spinner.Tick)
	case commandResultMsg:
		silent := msg.successMsg == "" && msg.failPrefix == ""
		if !silent {
			m.recordCommand(msg.elapsed, msg.err)
		}
		if msg.err != nil {
			if !silent {
				m = pushStatus(m, statusError, fmt.Sprintf("%s: %v", msg.failPrefix, msg.err))
			}
			return m, syncDeviceStateCmd(m.ip, m.port, true)
		}
		if msg.onSuccess != nil {
			msg.onSuccess(&m)
		}
		if !silent {
			m = pushStatus(m, statusSuccess, msg.successMsg)
		}
		return m, nil
	case fanoutResultMsg:
		m.recordCommand(msg.elapsed, nil)
		if len(msg.failed) == 0 {
			m = pushStatus(m, statusSuccess, fmt.Sprintf("%s (%d/%d ok)", msg.label, msg.ok, msg.ok))
		} else {
			m.commandFailed++
			m = pushStatus(m, statusError, fmt.Sprintf("%s (%d ok, failed: %s)", msg.label, msg.ok, strings.Join(msg.failed, ", ")))
		}
		return m, nil
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
					m = pushStatus(m, statusSuccess, "Config saved")
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
			case "h", "[":
				cmds = append(cmds, m.adjustBrightnessCmd(-10))
			case "l", "]":
				cmds = append(cmds, m.adjustBrightnessCmd(10))
			case "-", "_":
				cmds = append(cmds, m.adjustBrightnessCmd(-1))
			case "+", "=":
				cmds = append(cmds, m.adjustBrightnessCmd(1))
			case "enter", " ":
				m.lastKeyWasG = false
				switch m.cursor {
				case 0: // Toggle Power
					target := !m.isOn
					statusMsg := "Power: OFF"
					if target {
						statusMsg = "Power: ON"
					}
					if m.activeGroup != "" {
						// Group fan-out has no onSuccess hook, so flip optimistically
						// at dispatch (same pattern as adjustBrightnessCmd).
						m.isOn = target
					}
					cmds = append(cmds, m.sendToTarget("setState",
						map[string]interface{}{"state": target},
						statusMsg, "Power toggle failed",
						func(mm *model) { mm.isOn = target }))
				case 1: // Scenes
					m.state = scenesView
				case 2: // Groups
					m.state = groupsView
				case 3: // Color Grid
					m.state = colorPickerView
				case 4: // Hex Colors
					m.state = hexInputView
					m.textInput.CharLimit = 7
					m.textInput.Placeholder = "#CBA6F7"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 5: // Brightness
					m.state = brightnessView
				case 6: // Color Temp
					m.state = colorTempView
				case 7: // Sleep Timer
					m.state = timerInputView
					m.textInput.CharLimit = 5
					m.textInput.Placeholder = "Mins (e.g. 15)"
					m.textInput.SetValue("")
					m.textInput.Focus()
				case 8: // Discover Devices
					m.state = discoveryView
					m.discovering = true
					m = pushStatus(m, statusInfo, "Scanning local network...")
					cmds = append(cmds, discoverDevicesCmd(), m.spinner.Tick)
				case 9: // Saved Devices
					m.state = savedDevicesView
				case 10: // Theme
					m.state = themesView
				case 11: // Help
					m.state = helpView
				case 12: // Exit
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
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
						map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
						"", "", nil)) // silent preview
				}
			case "down", "j":
				if m.colorCursor < len(colorPalette)-3 {
					m.colorCursor += 3
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
						map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
						"", "", nil)) // silent preview
				}
			case "left", "h":
				if m.colorCursor > 0 {
					m.colorCursor--
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
						map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
						"", "", nil)) // silent preview
				}
			case "right", "l":
				if m.colorCursor < len(colorPalette)-1 {
					m.colorCursor++
					r, g, b, _ := wiz.HexToRGB(colorPalette[m.colorCursor].hex)
					cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
						map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
						"", "", nil)) // silent preview
				}
			case "enter":
				selectedHex := colorPalette[m.colorCursor].hex
				selectedName := colorPalette[m.colorCursor].name
				r, g, b, _ := wiz.HexToRGB(selectedHex)
				cmds = append(cmds, m.sendToTarget("setPilot",
					map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
					"Color: "+selectedName, "Color change failed",
					func(mm *model) {
						mm.currentColor = selectedHex
						mm.isOn = true
						mm.persistConfig()
					}))
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
					m = pushStatus(m, statusError, "Err: Invalid Hex")
				} else {
					normalized := normalizeHex(val)
					cmds = append(cmds, m.sendToTarget("setPilot",
						map[string]interface{}{"r": r, "g": g, "b": b, "dimming": m.brightness},
						"Color: "+normalized, "Color change failed",
						func(mm *model) {
							mm.currentColor = normalized
							mm.isOn = true
							mm.persistConfig()
						}))
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
			case "left", "h", "[":
				cmds = append(cmds, m.adjustBrightnessCmd(-10))
			case "right", "l", "]":
				cmds = append(cmds, m.adjustBrightnessCmd(10))
			case "-", "_":
				cmds = append(cmds, m.adjustBrightnessCmd(-1))
			case "+", "=":
				cmds = append(cmds, m.adjustBrightnessCmd(1))
			}
		case colorTempView:
			sendColorTemp := func(k int) {
				m.colorTemp = k
				cmds = append(cmds, m.sendToTarget("setPilot",
					map[string]interface{}{"temp": k, "dimming": m.brightness},
					fmt.Sprintf("Temp: %dK", k), "Color temp failed",
					func(mm *model) { mm.persistConfig() }))
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
					m = pushStatus(m, statusInfo, fmt.Sprintf("Sleep in %dm", mins))
					cmds = append(cmds, startTimer(time.Duration(mins)*time.Minute), m.spinner.Tick)
					if spawnErr := startDetachedTimer(mins, m.ip, m.port); spawnErr == nil {
						m.detachedTimer = true
						m = pushStatus(m, statusInfo, fmt.Sprintf("Sleep in %dm (background armed)", mins))
					} else {
						m = pushStatus(m, statusError, fmt.Sprintf("Sleep in %dm (local only): %v", mins, spawnErr))
					}
				} else {
					m = pushStatus(m, statusError, "Invalid timer value")
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
					m = pushStatus(m, statusInfo, "Rescanning local network...")
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
					m.activeGroup = ""
					m.persistConfig()
					m = pushStatus(m, statusInfo, fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.IP))
					m.state = menuView
					m.syncingState = true
					cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port, false), m.spinner.Tick)
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
					m.activeGroup = ""
					m.persistConfig()
					m = pushStatus(m, statusInfo, fmt.Sprintf("Selected saved device: %s", selected.Name))
					m.state = menuView
					if m.activeMac != "" {
						// Fast MAC-based resolution in background
						m.syncingState = true
						cmds = append(cmds, resolveDeviceCmd(m.activeMac, m.port), m.spinner.Tick)
					} else {
						m.syncingState = true
						cmds = append(cmds, syncDeviceStateCmd(m.ip, m.port, false), m.spinner.Tick)
					}
				}
			case "d":
				if len(m.savedDevices) > 0 {
					name := m.savedDevices[m.savedDeviceCursor].Name
					m.deleteSavedDevice()
					m.persistConfig()
					m = pushStatus(m, statusInfo, fmt.Sprintf("Removed saved device: %s", name))
				}
			}
		case saveDeviceNameView:
			switch msg.String() {
			case "esc":
				m.textInput.Blur()
				m.state = discoveryView
			case "enter":
				if strings.TrimSpace(m.pendingSaveDevice.Mac) == "" {
					m = pushStatus(m, statusError, "Cannot save device without MAC")
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
				m = pushStatus(m, statusSuccess, fmt.Sprintf("Saved device: %s", saved.Name))
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
		case scenesView:
			preview := func() {
				cmds = append(cmds, sendCmd(m.ip, m.port, "setPilot",
					sceneParams(scenes[m.sceneCursor].id), "", "", nil)) // silent preview
			}
			apply := func(idx int) {
				s := scenes[idx]
				cmds = append(cmds, m.sendToTarget("setPilot", sceneParams(s.id),
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
				if m.editingGroup < 0 {
					m.textInput.CharLimit = 32
					m.textInput.Placeholder = "Group name"
					m.textInput.SetValue("")
					m.textInput.Focus()
					m.state = groupNameView
				}
			case "e":
				if m.editingGroup < 0 && len(m.groups) > 0 {
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
		}
	}
	return m, tea.Batch(cmds...)
}
