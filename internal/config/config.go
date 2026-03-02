package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// Config stores target bulb network settings.
type Config struct {
	IP           string        `json:"ip"`
	Port         string        `json:"port"`
	SavedDevices []SavedDevice `json:"savedDevices,omitempty"`
}

// SavedDevice stores a user-named bulb target for quick reuse.
type SavedDevice struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port string `json:"port"`
	Mac  string `json:"mac,omitempty"`
}

// Path returns the persisted config file location in the user home directory.
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lumina-config.json")
}

// Validate verifies IP and port values are valid and usable.
func Validate(ip, port string) error {
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

// Load reads config from disk.
func Load() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(Path())
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes config to disk.
func Save(cfg Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0644)
}
