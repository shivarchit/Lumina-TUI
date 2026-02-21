package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	// Handle version flag before starting the TUI
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("Lumina-TUI %s\n", Version)
			os.Exit(0)
		}
	}

	// Load environment variables from .env file
	_ = godotenv.Load()
	
	// Get IP from environment or use default
	ip := os.Getenv("WIZ_IP")
	if ip == "" { 
		ip = "192.168.1.2" 
	}
	
	// Get Port from environment or use default
	port := os.Getenv("WIZ_PORT")
	if port == "" { 
		port = "38899" 
	}

	// Initialize the Bubble Tea program with the AltScreen (full-screen mode)
	p := tea.NewProgram(initialModel(ip, port), tea.WithAltScreen())
	
	// Run the TUI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Lumina-TUI: %v\n", err)
		os.Exit(1)
	}
}