package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	
	ip := os.Getenv("WIZ_IP")
	if ip == "" { ip = "192.168.1.2" }
	
	port := os.Getenv("WIZ_PORT")
	if port == "" { port = "38899" }

	p := tea.NewProgram(initialModel(ip, port), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Lumina-TUI: %v\n", err)
		os.Exit(1)
	}
}