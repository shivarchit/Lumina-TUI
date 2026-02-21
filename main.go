package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

// Config holds the saved IP and Port for binary users
type Config struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lumina-config.json")
}

func main() {
	// Handle version flag before starting the TUI
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("Lumina-TUI %s\n", Version)
			os.Exit(0)
		}
	}

	configPath := getConfigPath()
	var cfg Config
	needsSetup := false

	// Try to read the JSON config file first (for binary users)
	file, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(file, &cfg)
	} else {
		// If no config file, fallback to checking .env (for developers)
		_ = godotenv.Load()
		cfg.IP = os.Getenv("WIZ_IP")
		cfg.Port = os.Getenv("WIZ_PORT")

		// If nothing is found in the environment either, trigger the setup screen!
		if cfg.IP == "" {
			needsSetup = true
		}
		if cfg.Port == "" {
			cfg.Port = "38899" // Fallback default
		}
	}

	// Initialize the Bubble Tea program with the setup flag
	p := tea.NewProgram(initialModel(cfg.IP, cfg.Port, needsSetup), tea.WithAltScreen())
	
	// Run the TUI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Lumina-TUI: %v\n", err)
		os.Exit(1)
	}
}