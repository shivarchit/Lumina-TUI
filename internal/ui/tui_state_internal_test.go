package ui

import (
	"testing"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
)

func TestActiveMac(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "exact IP match returns normalized MAC",
			cfg: config.Config{IP: "192.168.1.10", SavedDevices: []config.SavedDevice{
				{IP: "192.168.1.10", Mac: " AA:BB:CC:DD:EE:FF "},
			}},
			want: "aa:bb:cc:dd:ee:ff",
		},
		{
			name: "no IP match returns empty even if saved devices exist",
			cfg: config.Config{IP: "192.168.1.99", SavedDevices: []config.SavedDevice{
				{IP: "192.168.1.10", Mac: "aa:bb:cc:dd:ee:ff"},
			}},
			want: "",
		},
		{
			name: "no saved devices returns empty",
			cfg:  config.Config{IP: "192.168.1.10"},
			want: "",
		},
	}

	for _, tc := range cases {
		if got := activeMac(tc.cfg); got != tc.want {
			t.Fatalf("%s: activeMac = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestApplyThemeFallsBackToMocha(t *testing.T) {
	if got := applyTheme("nonsense"); got != "mocha" {
		t.Fatalf("unknown theme must resolve to mocha, got %q", got)
	}
	if got := applyTheme(""); got != "mocha" {
		t.Fatalf("empty theme must resolve to mocha, got %q", got)
	}
	if got := applyTheme("latte"); got != "latte" {
		t.Fatalf("known theme must resolve to itself, got %q", got)
	}
	if string(mauve) != themes["latte"].mauve {
		t.Fatalf("applyTheme must reassign package colors: mauve=%s want %s", mauve, themes["latte"].mauve)
	}
	applyTheme("mocha") // restore for other tests
}

func TestSceneParams(t *testing.T) {
	p := sceneParams(4)
	if p["sceneId"] != 4 {
		t.Fatalf("sceneParams(4) = %v", p)
	}
	if len(scenes) != 12 {
		t.Fatalf("expected 12 curated scenes, got %d", len(scenes))
	}
	if scenes[0].name != "Ocean" || scenes[0].id != 1 {
		t.Fatalf("first scene must be Ocean/1, got %+v", scenes[0])
	}
}

func TestGroupTargetsResolvesMacsToIPs(t *testing.T) {
	m := testModel()
	m.savedDevices = []config.SavedDevice{
		{Name: "Desk", IP: "10.0.0.2", Port: "38899", Mac: "aa:aa"},
		{Name: "Shelf", IP: "10.0.0.3", Mac: "bb:bb"},
	}
	m.groups = []config.Group{{Name: "Room", Macs: []string{"aa:aa", "bb:bb", "cc:cc"}}}
	m.activeGroup = "Room"
	targets, names := m.groupTargets()
	if len(targets) != 2 || len(names) != 2 {
		t.Fatalf("expected 2 resolvable members, got %d/%d", len(targets), len(names))
	}
	if targets[0][0] != "10.0.0.2" || targets[0][1] != "38899" {
		t.Fatalf("bad first target: %v", targets[0])
	}
	if targets[1][1] != m.port {
		t.Fatalf("missing member port must fall back to model port, got %q", targets[1][1])
	}
}
