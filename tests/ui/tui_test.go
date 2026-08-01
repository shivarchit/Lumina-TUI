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

func TestInvalidHexShowsErrorStatus(t *testing.T) {
	var mod tea.Model = ui.NewModel(config.Config{IP: "192.168.1.5", Port: "38899"}, false)
	press := func(msg tea.Msg) {
		mod, _ = mod.Update(msg)
	}

	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")}) // jump cursor to Hex Colors
	press(tea.KeyMsg{Type: tea.KeyEnter})                     // open hex input
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")}) // type invalid value
	press(tea.KeyMsg{Type: tea.KeyEnter})                     // submit

	view := mod.View()
	if !strings.Contains(view, "Error") {
		t.Fatalf("expected Error badge after invalid hex, view: %q", view)
	}
}

func TestColorTempViewRendersPanel(t *testing.T) {
	var mod tea.Model = ui.NewModel(config.Config{IP: "192.168.1.5", Port: "38899"}, false)
	press := func(msg tea.Msg) {
		mod, _ = mod.Update(msg)
	}

	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")}) // jump to Color Temp
	press(tea.KeyMsg{Type: tea.KeyEnter})                     // open it

	view := mod.View()
	if !strings.Contains(view, "Color Temp") {
		t.Fatalf("color temp view must render a titled panel, got blank left panel: %q", view[:200])
	}
}
