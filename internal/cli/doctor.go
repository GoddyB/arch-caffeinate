package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/GoddyB/arch-caffeinate/internal/core"
	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func doctor(ctx context.Context, opts Options, paths daemon.Paths) error {
	fail := 0
	line := func(level, msg string) {
		fmt.Fprintf(opts.Stdout, "%s %s\n", level, msg)
		if level == "FAIL" {
			fail++
		}
	}
	if opts.GOOS != "darwin" {
		line("FAIL", "macOS: not macOS")
	} else {
		line("PASS", "macOS")
	}
	bin, err := opts.LoginLookup()
	if err != nil || bin == "" {
		line("FAIL", "binary: arch-caffeinate is not on PATH in a login bash")
	} else {
		line("PASS", "binary: "+bin)
	}
	if _, err := opts.Launch(ctx, "print", serviceTarget(opts.UID)); err != nil {
		line("FAIL", "launchagent: not loaded")
	} else {
		line("PASS", "launchagent: loaded")
	}
	saved, err := daemon.ReadState(paths)
	if err == nil && saved.Live(opts.Now()) {
		line("PASS", "heartbeat: fresh")
	} else {
		line("FAIL", "heartbeat: stale or missing")
	}
	reading := daemon.Snapshot(ctx, opts.Host)
	if reading.PowerErr != nil {
		line("FAIL", "power: not readable")
	} else {
		line("PASS", "power: readable")
	}
	if reading.IdleErr != nil {
		line("FAIL", "idle: not readable")
	} else {
		line("PASS", "idle: readable")
	}
	switch {
	case reading.LockErr != nil:
		line("FAIL", "screenlock: not readable")
	case reading.Lock == core.ScreenLockUnknown:
		line("WARN", "screenlock: unknown")
	default:
		line("PASS", "screenlock: "+string(reading.Lock))
	}
	if fail > 0 {
		return errDoctor
	}
	return nil
}

var errDoctor = errors.New("doctor failed")
