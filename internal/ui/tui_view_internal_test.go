package ui

import (
	"strings"
	"testing"
)

func TestRenderStatusBlockUsesExplicitLevel(t *testing.T) {
	out := renderStatusBlock("Device not found on network: timeout", statusError, 30)
	if !strings.Contains(out, "Error") {
		t.Fatalf("expected Error badge, got: %q", out)
	}
	if strings.Contains(out, "Success") {
		t.Fatalf("error status must not render as Success: %q", out)
	}

	out = renderStatusBlock("Ready.", statusInfo, 30)
	if !strings.Contains(out, "Info") {
		t.Fatalf("expected Info badge, got: %q", out)
	}
}
