package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

type wizPayload struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// sendCommand dials the UDP address and sends the JSON payload
func sendCommand(ip, port, method string, params map[string]interface{}) {
	payload := wizPayload{Method: method, Params: params}
	jsonData, _ := json.Marshal(payload)
	conn, err := net.Dial("udp", ip+":"+port)
	if err == nil {
		defer conn.Close()
		conn.Write(jsonData)
	}
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