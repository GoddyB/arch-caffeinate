package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

type savedDaemon struct {
	File      daemon.StateFile
	Live      bool
	Threshold int
}

func savedFacts(paths daemon.Paths, now time.Time) savedDaemon {
	threshold := installedIdle(paths)
	rec, err := daemon.ReadState(paths)
	if err != nil || !rec.Live(now) {
		return savedDaemon{Threshold: threshold}
	}
	if rec.IdleThresholdSeconds > 0 {
		threshold = rec.IdleThresholdSeconds
	}
	return savedDaemon{File: rec, Live: true, Threshold: threshold}
}

func status(ctx context.Context, opts Options, paths daemon.Paths, asJSON bool) error {
	reading := daemon.Snapshot(ctx, opts.Host)
	saved := savedFacts(paths, opts.Now())
	st := reading.Status()
	st.Running = saved.Live
	st.SleepPrevented = saved.Live && saved.File.SleepPrevented
	st.IdleThresholdSeconds = saved.Threshold
	st.Version = daemon.Version
	var pid *int
	if saved.Live {
		pid = saved.File.PID
		st.PID = pid
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
		st.Running, pidText, st.Power, st.SleepPrevented, st.Display, st.IdleSeconds, st.IdleThresholdSeconds, st.ScreenLock, st.Version)
	return nil
}
