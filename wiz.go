package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// wizPayload represents the JSON structure sent to WiZ devices
type wizPayload struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// discoveryPayload is the payload used for device discovery
type discoveryPayload struct {
	Method string            `json:"method"`
	Params map[string]string `json:"params"`
}

// Device represents a discovered WiZ device
type Device struct {
	IP   string
	Mac  string
	Name string
}

// discoverDevices attempts to find WiZ devices on the local network
func discoverDevices() ([]Device, error) {
	var devices []Device

	// Create UDP connection for broadcasting
	// Note: This broadcasts to 255.255.255.255:38899 which works on both 2.4GHz and 5GHz
	// as long as devices are on the same subnet
	conn, err := net.Dial("udp", "255.255.255.255:38899")
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery socket: %w", err)
	}
	defer conn.Close()

	// Set broadcast option
	if udpConn, ok := conn.(*net.UDPConn); ok {
		udpConn.SetWriteBuffer(1024)
	}

	// Send discovery payload - WiZ devices listen on UDP port 38899
	discovery := discoveryPayload{
		Method: "getSystemConfig",
		Params: map[string]string{},
	}

	jsonData, err := json.Marshal(discovery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery payload: %w", err)
	}

	// Broadcast to network - this reaches all devices on the same subnet
	// regardless of whether they're on 2.4GHz or 5GHz (same IP network)
	_, err = conn.Write(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to send discovery broadcast: %w", err)
	}

	// Listen for responses (with timeout)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	buffer := make([]byte, 1024)
	for {
		udpConn := conn.(*net.UDPConn)
		n, addr, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break // Timeout is expected
			}
			return nil, fmt.Errorf("error reading discovery response: %w", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			continue // Skip invalid responses
		}

		// Extract device info from WiZ response
		if result, ok := response["result"].(map[string]interface{}); ok {
			if mac, ok := result["mac"].(string); ok {
				device := Device{
					IP:   addr.IP.String(),
					Mac:  mac,
					Name: fmt.Sprintf("WiZ-%s", mac[len(mac)-4:]), // Use last 4 chars of MAC
				}
				devices = append(devices, device)
			}
		}
	}

	return devices, nil
}

// sendCommand dials the UDP address and sends the JSON payload with retry logic
func sendCommand(ip, port, method string, params map[string]interface{}) error {
	payload := wizPayload{Method: method, Params: params}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	address := net.JoinHostPort(ip, port)
	
	// Retry up to 3 times with exponential backoff
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		conn, err := net.Dial("udp", address)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s (attempt %d): %w", address, attempt+1, err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
			continue
		}
		
		defer conn.Close()
		
		// Set a reasonable timeout
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		
		_, err = conn.Write(jsonData)
		if err != nil {
			lastErr = fmt.Errorf("failed to send data to %s (attempt %d): %w", address, attempt+1, err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
			continue
		}
		
		// Success
		return nil
	}
	
	return lastErr
}

// hexToRGB converts a hex string (e.g. #FF0000) to RGB integers
func hexToRGB(h string) (uint8, uint8, uint8, error) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex")
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0, 0, 0, err
	}
	return b[0], b[1], b[2], nil
}