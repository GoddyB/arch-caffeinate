package macos

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

func sh(script string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", script)
	}
}

func TestScreenLockReadsStatusPhrase(t *testing.T) {
	s := &System{execCmd: sh("echo 'sysadminctl -screenLock {status | immediate | off | seconds}' >&2; echo 'screenLock delay is immediate' >&2")}
	got, err := s.ScreenLock(context.Background())
	if err != nil || got != core.ScreenLockImmediate {
		t.Fatalf("got %s err %v", got, err)
	}
	s.execCmd = sh("echo 'sysadminctl -screenLock off' >&2")
	got, err = s.ScreenLock(context.Background())
	if err != nil || got != core.ScreenLockUnknown {
		t.Fatalf("usage got %s err %v", got, err)
	}
}

func TestSleepClearsWhenChildExits(t *testing.T) {
	s := &System{execCmd: sh("exit 0")}
	if err := s.AcquireSleep(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for s.SleepHeld() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.SleepHeld() {
		t.Fatal("sleep still held")
	}
}
