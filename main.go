package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// --- Styling ---
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1).MarginBottom(1)
	itemStyle  = lipgloss.NewStyle().PaddingLeft(2)
	selected   = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true).PaddingLeft(2)
)

// --- UDP Network Logic ---
type wizPayload struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

func sendCommand(ip, port, method string, params map[string]interface{}) {
	payload := wizPayload{
		Method: method,
		Params: params,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 
	}

	conn, err := net.Dial("udp", ip+":"+port)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.Write(jsonData)
}

// --- Bubble Tea App Logic ---
type model struct {
	choices []string
	cursor  int
	status  string
	ip      string
	port    string
}

func initialModel(ip, port string) model {
	return model{
		choices: []string{"Turn ON", "Turn OFF", "Set Color: Warm White", "Set Color: Ocean Blue", "Exit"},
		status:  "Targeting " + ip + ":" + port,
		ip:      ip,
		port:    port,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
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
				sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": true})
				m.status = "Light turned ON"
			case 1:
				sendCommand(m.ip, m.port, "setState", map[string]interface{}{"state": false})
				m.status = "Light turned OFF"
			case 2:
				sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"temp": 2700, "dimming": 100})
				m.status = "Color set to Warm White"
			case 3:
				sendCommand(m.ip, m.port, "setPilot", map[string]interface{}{"r": 0, "g": 105, "b": 148, "dimming": 100})
				m.status = "Color set to Ocean Blue"
			case 4:
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render("💡 WiZ Light Controller") + "\n"

	for i, choice := range m.choices {
		cursor := " " 
		if m.cursor == i {
			cursor = ">" 
			s += selected.Render(fmt.Sprintf("%s %s", cursor, choice)) + "\n"
		} else {
			s += itemStyle.Render(fmt.Sprintf("%s %s", cursor, choice)) + "\n"
		}
	}

	s += fmt.Sprintf("\n[ %s ]\n", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.status))
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("\nPress 'q' to quit, 'Up/Down' to navigate, 'Enter' to select.")

	return s
}

func main() {
	// Attempt to load the .env file. We ignore errors so it doesn't break the UI if the file is missing
	_ = godotenv.Load()

	// Fetch variables from the environment, with fallbacks just in case
	ip := os.Getenv("WIZ_IP")
	if ip == "" {
		ip = "192.168.1.2" 
	}

	port := os.Getenv("WIZ_PORT")
	if port == "" {
		port = "38899" 
	}

	p := tea.NewProgram(initialModel(ip, port), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}