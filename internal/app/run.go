package app

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"wiz-tui/internal/config"
	"wiz-tui/internal/ui"
	"wiz-tui/internal/version"
	"wiz-tui/internal/wiz"
)

// Run executes CLI handling and starts the interactive Lumina TUI.
func Run() {
	var (
		timer    = flag.Int("timer", 0, "sleep timer in minutes; if >0 program will wait and then send a power command")
		ipFlag   = flag.String("ip", "", "target device IP address (required when --timer > 0)")
		portFlag = flag.String("port", "38899", "target device UDP port")
		offFlag  = flag.Bool("off", false, "when used with --timer the command will turn the light off (default)")
	)

	flag.Parse()

	for _, a := range os.Args[1:] {
		if a == "-v" || a == "--version" || a == "version" {
			fmt.Printf("Lumina-TUI %s\n", version.Version)
			os.Exit(0)
		}
	}

	if *timer > 0 {
		if err := config.Validate(*ipFlag, *portFlag); err != nil {
			fmt.Fprintf(os.Stderr, "invalid timer configuration: %v\n", err)
			os.Exit(1)
		}
		dur := time.Duration(*timer) * time.Minute
		fmt.Printf("sleep timer: %dm -> %s:%s (off=%v)\n", *timer, *ipFlag, *portFlag, *offFlag)
		time.Sleep(dur)
		state := !*offFlag
		if err := wiz.SendCommand(*ipFlag, *portFlag, "setState", map[string]interface{}{"state": state}); err != nil {
			fmt.Fprintf(os.Stderr, "timer command failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("timer command sent")
		os.Exit(0)
	}

	cfg, needsSetup := loadRuntimeConfig()
	if cfg.Port == "" {
		cfg.Port = "38899"
	}

	if !needsSetup {
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setState", map[string]interface{}{"state": true}); err != nil {
			fmt.Printf("Warning: auto power-on failed: %v\n", err)
		}
	}

	p := tea.NewProgram(ui.NewModel(cfg, needsSetup), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Lumina-TUI: %v\n", err)
		os.Exit(1)
	}
}

// loadRuntimeConfig loads saved config first, then falls back to environment values.
func loadRuntimeConfig() (config.Config, bool) {
	cfg, err := config.Load()
	if err == nil {
		if cfg.Port == "" {
			cfg.Port = "38899"
		}
		if cfg.IP == "" && len(cfg.SavedDevices) > 0 {
			cfg.IP = cfg.SavedDevices[0].IP
			if cfg.SavedDevices[0].Port != "" {
				cfg.Port = cfg.SavedDevices[0].Port
			}
		}
		if validErr := config.Validate(cfg.IP, cfg.Port); validErr == nil {
			return cfg, false
		}
		if len(cfg.SavedDevices) > 0 {
			for _, saved := range cfg.SavedDevices {
				if saved.IP == "" {
					continue
				}
				port := saved.Port
				if port == "" {
					port = cfg.Port
				}
				if validErr := config.Validate(saved.IP, port); validErr == nil {
					cfg.IP = saved.IP
					cfg.Port = port
					return cfg, false
				}
			}
		}
		fmt.Printf("Warning: Invalid config and no valid saved device target, running setup...\n")
		return cfg, true
	}

	_ = godotenv.Load()
	cfg.IP = os.Getenv("WIZ_IP")
	cfg.Port = os.Getenv("WIZ_PORT")
	if cfg.IP == "" || cfg.Port == "" {
		return cfg, true
	}
	if validErr := config.Validate(cfg.IP, cfg.Port); validErr != nil {
		fmt.Printf("Warning: Invalid environment config (%v), running setup...\n", validErr)
		return cfg, true
	}
	return cfg, false
}
