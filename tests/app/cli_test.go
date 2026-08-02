package app_test

import (
	"testing"

	"github.com/shivarchit/Lumina-TUI/internal/app"
)

func TestParseVerb(t *testing.T) {
	cases := []struct {
		args []string
		verb string
	}{
		{[]string{}, ""},
		{[]string{"on"}, "on"},
		{[]string{"off"}, "off"},
		{[]string{"color", "#FF0000"}, "color"},
		{[]string{"scene", "party"}, "scene"},
		{[]string{"status"}, "status"},
		{[]string{"--timer", "5"}, ""},
		{[]string{"-v"}, ""},
	}
	for _, tc := range cases {
		verb, _ := app.ParseVerb(tc.args)
		if verb != tc.verb {
			t.Fatalf("ParseVerb(%v) = %q, want %q", tc.args, verb, tc.verb)
		}
	}
}
