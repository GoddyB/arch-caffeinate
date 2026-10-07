package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func status(ctx context.Context, opts Options, paths daemon.Paths, asJSON bool) error {
	reading := daemon.Snapshot(ctx, opts.Host)
	running, pid, sleep, threshold := daemonFacts(paths)
	display := "on"
	if reading.DisplayErr == nil {
		display = reading.Display.String()
	}
	idle := 0.0
	if reading.IdleErr == nil {
		idle = reading.Idle.Seconds()
	}
	lock := "unknown"
	if reading.LockErr == nil {
		lock = string(reading.Lock)
	}
	st := daemon.Status{
		Running:              running,
		PID:                  pid,
		Power:                reading.Power.String(),
		SleepPrevented:       sleep,
		Display:              display,
		IdleSeconds:          idle,
		IdleThresholdSeconds: threshold,
		ScreenLock:           lock,
		Version:              daemon.Version,
	}
	if asJSON {
		b, err := json.Marshal(st)
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.Stdout, "%s\n", b)
		return nil
	}
	pidText := "null"
	if pid != nil {
		pidText = strconv.Itoa(*pid)
	}
	fmt.Fprintf(opts.Stdout, "running: %t\npid: %s\npower: %s\nsleepPrevented: %t\ndisplay: %s\nidleSeconds: %g\nidleThresholdSeconds: %d\nscreenLock: %s\nversion: %s\n",
		running, pidText, reading.Power.String(), sleep, display, idle, threshold, lock, daemon.Version)
	return nil
}

func daemonFacts(paths daemon.Paths) (bool, *int, bool, int) {
	threshold := installedIdle(paths)
	rec, err := daemon.ReadState(paths)
	if err != nil || rec.PID == nil || !alive(*rec.PID) {
		return false, nil, false, threshold
	}
	return true, rec.PID, rec.SleepPrevented, rec.IdleThresholdSeconds
}

func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
