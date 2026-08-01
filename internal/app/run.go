package app

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"wiz-tui/internal/config"
	"wiz-tui/internal/ui"
	"wiz-tui/internal/version"
	"wiz-tui/internal/wiz"
)

var cliVerbs = map[string]bool{"on": true, "off": true, "color": true, "temp": true, "scene": true, "status": true, "discover": true}

var sceneIDs = map[string]int{
	"ocean": 1, "romance": 2, "sunset": 3, "party": 4, "fireplace": 5, "cozy": 6,
	"forest": 7, "pastel": 8, "wake-up": 9, "bedtime": 10, "daylight": 12, "focus": 15,
}

// ParseVerb splits a CLI invocation into a known verb and its arguments.
// Unknown first args (flags, empty) mean "no verb": launch the TUI.
func ParseVerb(args []string) (string, []string) {
	if len(args) == 0 || !cliVerbs[args[0]] {
		return "", args
	}
	return args[0], args[1:]
}

// Run executes CLI handling and starts the interactive Lumina TUI.
func Run() {
	for _, a := range os.Args[1:] {
		if a == "-v" || a == "--version" || a == "version" {
			fmt.Printf("Lumina-TUI %s\n", version.Version)
			os.Exit(0)
		}
	}

	if verb, rest := ParseVerb(os.Args[1:]); verb != "" {
		runCLI(verb, rest)
		return
	}

	var (
		timer    = flag.Int("timer", 0, "sleep timer in minutes; if >0 program will wait and then send a power command")
		ipFlag   = flag.String("ip", "", "target device IP address (required when --timer > 0)")
		portFlag = flag.String("port", "38899", "target device UDP port")
		offFlag  = flag.Bool("off", false, "when used with --timer the command will turn the light off (default)")
	)

	flag.Parse()

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
		if validErr := config.Validate(cfg.IP, cfg.Port); validErr == nil {
			return cfg, false
		}
		if len(cfg.SavedDevices) > 0 {
			for _, saved := range cfg.SavedDevices {
				if saved.IP == "" || strings.TrimSpace(saved.Mac) == "" {
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

// runCLI executes a one-shot device command and exits.
func runCLI(verb string, args []string) {
	cfg, needsSetup := loadRuntimeConfig()
	if cfg.Port == "" {
		cfg.Port = "38899"
	}
	if needsSetup {
		fmt.Fprintln(os.Stderr, "no configured device - run lumina once to set up, or set WIZ_IP")
		os.Exit(1)
	}

	fail := func(err error) {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", verb, err)
		os.Exit(1)
	}

	switch verb {
	case "on", "off":
		state := verb == "on"
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setState", map[string]interface{}{"state": state}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → power %s\n", cfg.IP, cfg.Port, verb)
	case "color":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina color <#RRGGBB>"))
		}
		r, g, b, err := wiz.HexToRGB(args[0])
		if err != nil {
			fail(fmt.Errorf("invalid hex %q", args[0]))
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"r": r, "g": g, "b": b}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → color %s\n", cfg.IP, cfg.Port, args[0])
	case "temp":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina temp <2200-6500>"))
		}
		k, err := strconv.Atoi(args[0])
		if err != nil || k < 2200 || k > 6500 {
			fail(fmt.Errorf("kelvin must be 2200-6500, got %q", args[0]))
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"temp": k}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → temp %dK\n", cfg.IP, cfg.Port, k)
	case "scene":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: lumina scene <name|id>"))
		}
		id, ok := sceneIDs[strings.ToLower(args[0])]
		if !ok {
			if n, err := strconv.Atoi(args[0]); err == nil && n >= 1 && n <= 32 {
				id = n
			} else {
				fail(fmt.Errorf("unknown scene %q", args[0]))
			}
		}
		if err := wiz.SendCommand(cfg.IP, cfg.Port, "setPilot", map[string]interface{}{"sceneId": id}); err != nil {
			fail(err)
		}
		fmt.Printf("%s:%s → scene %s\n", cfg.IP, cfg.Port, args[0])
	case "status":
		targets := [][2]string{}
		for _, d := range cfg.SavedDevices {
			if d.IP != "" {
				port := d.Port
				if port == "" {
					port = cfg.Port
				}
				targets = append(targets, [2]string{d.IP, port})
			}
		}
		if len(targets) == 0 {
			targets = append(targets, [2]string{cfg.IP, cfg.Port})
		}
		for i, t := range targets {
			name := t[0]
			if i < len(cfg.SavedDevices) && cfg.SavedDevices[i].Name != "" {
				name = cfg.SavedDevices[i].Name
			}
			st, err := wiz.GetPilotState(t[0], t[1])
			if err != nil {
				fmt.Printf("%-16s unreachable: %v\n", name, err)
				continue
			}
			power := "off"
			if st.Power {
				power = "on"
			}
			detail := st.ColorHex
			if st.Temp > 0 && st.ColorHex == "" {
				detail = fmt.Sprintf("%dK", st.Temp)
			}
			fmt.Printf("%-16s %s · %d%% · %s\n", name, power, st.Brightness, detail)
		}
	case "discover":
		devices, err := wiz.DiscoverDevices()
		if err != nil {
			fail(err)
		}
		for _, d := range devices {
			fmt.Printf("%-16s %s  %s\n", d.Name, d.IP, d.Mac)
		}
		fmt.Printf("%d device(s)\n", len(devices))
	}
}
