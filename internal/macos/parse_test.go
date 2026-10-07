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
	if got := ParseScreenLock("screenLock delay is immediate\n"); got != "immediate" {
		t.Fatalf("got %s", got)
	}
	if got := ParseScreenLock("screenLock delay is off\n"); got != "off" {
		t.Fatalf("got %s", got)
	}
	if got := ParseScreenLock("screenLock delay is 300 seconds\n"); got != "delay:300" {
		t.Fatalf("got %s", got)
	}
	if got := ParseScreenLock("screenLock status unavailable\n"); got != "unknown" {
		t.Fatalf("got %s", got)
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
}

func TestParseNotification(t *testing.T) {
	line := `{"eventMessage":"Delivering com.apple.notification","process":"usernoted","subsystem":"com.apple.unc"}`
	if !ParseNotification(line) {
		t.Fatal("expected delivery")
	}
	if ParseNotification(`{"eventMessage":"Dropped note"}`) {
		t.Fatal("expected ignore")
	}
}
