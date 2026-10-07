package macos

import (
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

func TestParsePower(t *testing.T) {
	if got := ParsePower("Now drawing from 'AC Power'\n -InternalBattery- 0\n"); got != core.PowerAC {
		t.Fatalf("got %s", got)
	}
	if got := ParsePower("Now drawing from 'Battery Power'\n"); got != core.PowerBattery {
		t.Fatalf("got %s", got)
	}
	if got := ParsePower("Now drawing from 'UPS Power'\n"); got != core.PowerUnknown {
		t.Fatalf("got %s", got)
	}
	if got := ParsePower("Now drawing from 'UPS Power'\n -InternalBattery-0 'AC Power'\n"); got != core.PowerUnknown {
		t.Fatalf("got %s", got)
	}
}

func TestParseIdle(t *testing.T) {
	sample := "  |   \"HIDIdleTime\" = 1234567890\n"
	got, ok := ParseIdle(sample)
	if !ok || got != 1234567890*time.Nanosecond {
		t.Fatalf("got %s ok %v", got, ok)
	}
	if _, ok := ParseIdle("no idle here"); ok {
		t.Fatal("expected miss")
	}
}

func TestParseScreenLock(t *testing.T) {
	cases := []struct {
		in   string
		want core.ScreenLock
	}{
		{"screenLock delay is immediate\n", core.ScreenLockImmediate},
		{"screenLock delay is off\n", core.ScreenLockOff},
		{"screenLock delay is 300 seconds\n", core.ScreenLockDelay("300")},
		{"screenLock status unavailable\n", core.ScreenLockUnknown},
		{"sysadminctl -screenLock {status | immediate | off | seconds} -password password\n", core.ScreenLockUnknown},
		{"sysadminctl -screenLock {status | immediate | off | seconds}\nscreenLock delay is immediate\n", core.ScreenLockImmediate},
	}
	for _, tc := range cases {
		if got := ParseScreenLock(tc.in); got != tc.want {
			t.Fatalf("%q got %s want %s", tc.in, got, tc.want)
		}
	}
}

func TestParseDisplay(t *testing.T) {
	on := `"IOPowerManagement" = {"CurrentPowerState"=1}`
	off := `"IOPowerManagement" = {"CurrentPowerState"=0}`
	got, ok := ParseDisplay(on)
	if !ok || got != core.DisplayOn {
		t.Fatalf("on: %s %v", got, ok)
	}
	got, ok = ParseDisplay(off)
	if !ok || got != core.DisplayOff {
		t.Fatalf("off: %s %v", got, ok)
	}
	if _, ok := ParseDisplay("no panel"); ok {
		t.Fatal("expected miss")
	}
	got, ok = ParseDisplay(`"CurrentPowerState"=2`)
	if ok || got != core.DisplayOn {
		t.Fatalf("unknown: %s %v", got, ok)
	}
}

func TestParseNotification(t *testing.T) {
	line := `{"eventMessage":"Delivering com.apple.notification","process":"usernoted","subsystem":"com.apple.unc"}`
	if !ParseNotification(line) {
		t.Fatal("expected delivery")
	}
	if ParseNotification(`{"eventMessage":"Dropped note"}`) {
		t.Fatal("expected ignore")
	}
	if ParseNotification(`{"eventMessage":"Dropped","note":"Delivering x"}`) {
		t.Fatal("expected ignore")
	}
	if ParseNotification("Filtering the log data using ...") {
		t.Fatal("expected ignore")
	}
}
