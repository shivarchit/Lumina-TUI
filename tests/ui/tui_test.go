package ui_test

import (
	"strings"
	"testing"

	"wiz-tui/internal/config"
	"wiz-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewShowsSetupScreenWhenNeeded(t *testing.T) {
	m := ui.NewModel(config.Config{}, true)
	view := m.View()
	if !strings.Contains(view, "FIRST-TIME SETUP") {
		t.Fatalf("expected setup screen, got view: %q", view)
	}
}

func TestViewShowsMenuByDefault(t *testing.T) {
	m := ui.NewModel(config.Config{IP: "192.168.1.5", Port: "38899"}, false)
	view := m.View()
	if !strings.Contains(view, "Control Board") {
		t.Fatalf("expected menu control board section, got view: %q", view)
	}
}

func TestInitReturnsCommand(t *testing.T) {
	m := ui.NewModel(config.Config{}, true)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected non-nil init command")
	}
}

func TestUpdateCtrlCQuits(t *testing.T) {
	m := ui.NewModel(config.Config{}, true)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command for ctrl+c")
	}
}
