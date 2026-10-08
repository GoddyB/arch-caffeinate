package macos

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strconv"
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

func TestAcquireSleepUsesSystemAssertion(t *testing.T) {
	var got []string
	s := &System{execCmd: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		got = append([]string{name}, args...)
		return exec.CommandContext(ctx, "sleep", "30")
	}}
	if err := s.AcquireSleep(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.ReleaseSleep(context.Background()) })
	want := []string{"caffeinate", "-s", "-w", strconv.Itoa(os.Getpid())}
	if !slices.Equal(got, want) {
		t.Fatalf("args %v", got)
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
