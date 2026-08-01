package ui

import (
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"wiz-tui/internal/config"
	"wiz-tui/internal/wiz"
)

func testModel() model {
	return NewModel(config.Config{IP: "192.168.1.5", Port: "38899"}, false)
}

func TestCommandResultSuccessAppliesMutation(t *testing.T) {
	m := testModel()
	msg := commandResultMsg{
		elapsed:    5 * time.Millisecond,
		successMsg: "Power: ON",
		failPrefix: "Power toggle failed",
		onSuccess:  func(mm *model) { mm.isOn = true },
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if !um.isOn {
		t.Fatal("onSuccess mutation not applied")
	}
	if um.status != "Power: ON" || um.statusLevel != statusSuccess {
		t.Fatalf("expected success status, got %q level %d", um.status, um.statusLevel)
	}
	if um.commandTotal != 1 || um.commandFailed != 0 {
		t.Fatalf("telemetry not recorded: total=%d failed=%d", um.commandTotal, um.commandFailed)
	}
}

func TestCommandResultErrorSkipsMutation(t *testing.T) {
	m := testModel()
	m.isOn = false
	msg := commandResultMsg{
		err:        errors.New("timeout"),
		elapsed:    5 * time.Millisecond,
		successMsg: "Power: ON",
		failPrefix: "Power toggle failed",
		onSuccess:  func(mm *model) { mm.isOn = true },
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.isOn {
		t.Fatal("mutation must not run on error")
	}
	if um.statusLevel != statusError {
		t.Fatalf("expected error status level, got %d", um.statusLevel)
	}
	if um.commandFailed != 1 {
		t.Fatalf("failure not recorded: failed=%d", um.commandFailed)
	}
}

func TestCommandResultSilentSkipsStatusAndTelemetry(t *testing.T) {
	m := testModel()
	before := m.status
	msg := commandResultMsg{elapsed: 5 * time.Millisecond} // successMsg=="" && failPrefix=="" -> silent

	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.status != before {
		t.Fatalf("silent result must not change status, got %q", um.status)
	}
	if um.commandTotal != 0 {
		t.Fatalf("silent result must not record telemetry, total=%d", um.commandTotal)
	}
}

func TestNormalizeHex(t *testing.T) {
	cases := map[string]string{
		"cba6f7":  "#CBA6F7",
		"#cba6f7": "#CBA6F7",
		"#CBA6F7": "#CBA6F7",
	}
	for in, want := range cases {
		if got := normalizeHex(in); got != want {
			t.Fatalf("normalizeHex(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAdjustBrightnessCmdStepsImmediately(t *testing.T) {
	m := testModel()
	m.brightness = 50

	cmd := m.adjustBrightnessCmd(10)
	if cmd == nil {
		t.Fatal("expected a send command")
	}
	if m.brightness != 60 {
		t.Fatalf("brightness must step at dispatch, got %d", m.brightness)
	}

	// second step before any result arrives must build on the first
	_ = m.adjustBrightnessCmd(10)
	if m.brightness != 70 {
		t.Fatalf("rapid steps must accumulate, got %d", m.brightness)
	}
}

func TestQuietSyncSkipsStatusAndSetsMode(t *testing.T) {
	m := testModel()
	before := m.status
	msg := stateSyncResultMsg{
		state: wiz.PilotState{Power: true, Brightness: 50, Temp: 4000},
		quiet: true, elapsed: 5 * time.Millisecond,
	}
	updated, _ := m.Update(msg)
	um := updated.(model)
	if um.status != before {
		t.Fatalf("quiet sync must not push status, got %q", um.status)
	}
	if !um.whiteMode {
		t.Fatal("Temp>0 with no color must set whiteMode")
	}
	if !um.lastSyncOK || um.lastSyncAt.IsZero() {
		t.Fatal("sync bookkeeping not recorded")
	}
}

func TestCommandFailureTriggersResync(t *testing.T) {
	m := testModel()
	msg := commandResultMsg{err: errors.New("timeout"), elapsed: time.Millisecond,
		successMsg: "x", failPrefix: "y"}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("command failure must chain a re-sync command")
	}
}

func TestAdjustBrightnessCmdClamps(t *testing.T) {
	m := testModel()
	m.brightness = 95
	_ = m.adjustBrightnessCmd(10)
	if m.brightness != 100 {
		t.Fatalf("expected clamp to 100, got %d", m.brightness)
	}

	m.brightness = 5
	_ = m.adjustBrightnessCmd(-10)
	if m.brightness != 1 {
		t.Fatalf("expected clamp to 1, got %d", m.brightness)
	}
}

func TestFanoutCmdAggregates(t *testing.T) {
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, e := srv.ReadFromUDP(buf); e != nil {
				return
			}
		}
	}()
	port := strconv.Itoa(srv.LocalAddr().(*net.UDPAddr).Port)

	targets := [][2]string{{"127.0.0.1", port}, {"127.0.0.1", port}}
	names := []string{"A", "B"}
	msg := fanoutCmd(targets, names, "setState", map[string]interface{}{"state": true}, "Room → on")()
	res, ok := msg.(fanoutResultMsg)
	if !ok {
		t.Fatalf("expected fanoutResultMsg, got %T", msg)
	}
	if res.ok != 2 || len(res.failed) != 0 {
		t.Fatalf("expected 2 ok / 0 failed, got %d/%v", res.ok, res.failed)
	}
}
