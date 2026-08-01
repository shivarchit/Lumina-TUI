package ui

import (
	"testing"

	"wiz-tui/internal/config"
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
