package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

// validateConfig checks if the IP and port are valid
func validateConfig(ip, port string) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address format: %s", ip)
	}

	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be a number: %s", port)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535: %d", portNum)
	}

	return nil
}

func main() {
	// --- command‑line helpers ------------------------------------------------
	// Support a simple timer worker mode.  The TUI will spawn a detached
	// copy of itself when the user sets a sleep timer so that the UDP
	// "power off" command can still fire even after the main interface quits.
	//
	// Example:
	//   lumina --timer 15 --ip 192.168.1.2 --port 38899 --off
	//
	// Any invocation with a non‑zero -timer flag will run the worker logic
	// below and then exit; the rest of the TUI setup is skipped.

	var (
		timer    = flag.Int("timer", 0, "sleep timer in minutes; if >0 program will wait and then send a power command")
		ipFlag   = flag.String("ip", "", "target device IP address (required when --timer > 0)")
		portFlag = flag.String("port", "38899", "target device UDP port")
		offFlag  = flag.Bool("off", false, "when used with --timer the command will turn the light off (default)")
	)

	flag.Parse()

	// version flag takes precedence over everything else
	for _, a := range os.Args[1:] {
		if a == "-v" || a == "--version" || a == "version" {
			fmt.Printf("Lumina-TUI %s\n", Version)
			os.Exit(0)
		}
	}

	if *timer > 0 {
		// worker mode
		if err := validateConfig(*ipFlag, *portFlag); err != nil {
			fmt.Fprintf(os.Stderr, "invalid timer configuration: %v\n", err)
			os.Exit(1)
		}
		dur := time.Duration(*timer) * time.Minute
		fmt.Printf("sleep timer: %dm -> %s:%s (off=%v)\n", *timer, *ipFlag, *portFlag, *offFlag)
		time.Sleep(dur)
		state := !*offFlag // if offFlag true we send state=false
		if err := sendCommand(*ipFlag, *portFlag, "setState", map[string]interface{}{"state": state}); err != nil {
			fmt.Fprintf(os.Stderr, "timer command failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(1)
		fmt.Println("timer command sent")
		os.Exit(1)
	}

	// ------------------------------------------------------------------------

	configPath := getConfigPath()
	var cfg Config
	needsSetup := false

	// Try to read the JSON config file first (for binary users)
	file, err := os.ReadFile(configPath)
	if err == nil {
		if jsonErr := json.Unmarshal(file, &cfg); jsonErr != nil {
			fmt.Printf("Warning: Could not parse config file %s: %v\n", configPath, jsonErr)
			needsSetup = true
		} else {
			// Validate loaded config
			if err := validateConfig(cfg.IP, cfg.Port); err != nil {
				fmt.Printf("Warning: Invalid config (%v), running setup...\n", err)
				needsSetup = true
			}
		}
	} else {
		// If no config file, fallback to checking .env (for developers)
		_ = godotenv.Load()
		cfg.IP = os.Getenv("WIZ_IP")
		cfg.Port = os.Getenv("WIZ_PORT")

		// If nothing is found in the environment either, trigger the setup screen!
		if cfg.IP == "" || cfg.Port == "" {
			needsSetup = true
		} else {
			// Validate environment config
			if err := validateConfig(cfg.IP, cfg.Port); err != nil {
				fmt.Printf("Warning: Invalid environment config (%v), running setup...\n", err)
				needsSetup = true
			}
		}
	}

	// Set default port if empty
	if cfg.Port == "" {
		cfg.Port = "38899"
	}

	// Initialize the Bubble Tea program with the setup flag
	p := tea.NewProgram(initialModel(cfg.IP, cfg.Port, needsSetup), tea.WithAltScreen())

	// Run the TUI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Lumina-TUI: %v\n", err)
		os.Exit(1)
	}
}
