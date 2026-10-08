package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

const (
	Version            = "0.1.0"
	Label              = "io.github.goddyb.arch-caffeinate"
	DefaultIdleSeconds = 600
	DefaultPollMs      = 1000
)

type Status struct {
	Running              bool    `json:"running"`
	PID                  *int    `json:"pid"`
	Power                string  `json:"power"`
	SleepPrevented       bool    `json:"sleepPrevented"`
	Display              string  `json:"display"`
	IdleSeconds          float64 `json:"idleSeconds"`
	IdleThresholdSeconds int     `json:"idleThresholdSeconds"`
	ScreenLock           string  `json:"screenLock"`
	Version              string  `json:"version"`
}

type StateFile struct {
	Status
	WrittenAt int64 `json:"writtenAt"`
	PollMs    int   `json:"pollMs"`
}

type Paths struct {
	Home string
}

func (p Paths) StateFile() string {
	return filepath.Join(p.Home, "Library", "Application Support", "arch-caffeinate", "state.json")
}

func (p Paths) LogFile() string {
	return filepath.Join(p.Home, "Library", "Logs", "arch-caffeinate.log")
}

func (p Paths) Plist() string {
	return filepath.Join(p.Home, "Library", "LaunchAgents", Label+".plist")
}

func (p Paths) Bin() string {
	return filepath.Join(p.Home, ".local", "bin", "arch-caffeinate")
}

type Config struct {
	IdleThreshold time.Duration
	Poll          time.Duration
	Now           func() time.Time
	Logf          func(string, ...any)
	seen          map[string]string
}

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c Config) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

func (c Config) report(name string, err error) {
	if c.seen == nil {
		if err != nil {
			c.logf("%s: %v", name, err)
		}
		return
	}
	if err == nil {
		if _, ok := c.seen[name]; ok {
			delete(c.seen, name)
			c.logf("%s: cleared", name)
		}
		return
	}
	msg := err.Error()
	if c.seen[name] == msg {
		return
	}
	c.seen[name] = msg
	c.logf("%s: %v", name, err)
}

type Reader interface {
	Power(context.Context) (core.Power, error)
	Idle(context.Context) (time.Duration, error)
	Display(context.Context) (core.Display, error)
	ScreenLock(context.Context) (core.ScreenLock, error)
	SleepHeld() bool
}

type Host interface {
	Reader
	TakeNotification() bool
	AcquireSleep(context.Context) error
	ReleaseSleep(context.Context) error
	DisplayOff(context.Context) error
	DeclareActivity(context.Context) error
	StartNotifications(context.Context, func(string, ...any)) (<-chan error, error)
}

type Reading struct {
	Power      core.Power
	PowerErr   error
	Idle       time.Duration
	IdleErr    error
	Display    core.Display
	DisplayErr error
	Lock       core.ScreenLock
	LockErr    error
}

func Snapshot(ctx context.Context, host Reader) Reading {
	var r Reading
	var err error
	r.Power, err = host.Power(ctx)
	if err != nil {
		r.Power = core.PowerUnknown
		r.PowerErr = err
	}
	r.Idle, err = host.Idle(ctx)
	if err != nil {
		r.Idle = 0
		r.IdleErr = err
	}
	r.Display, err = host.Display(ctx)
	if err != nil {
		r.Display = core.DisplayOn
		r.DisplayErr = err
	}
	r.Lock, err = host.ScreenLock(ctx)
	if err != nil {
		r.Lock = core.ScreenLockUnknown
		r.LockErr = err
	}
	return r
}

func (r Reading) Status() Status {
	return Status{
		Power:       r.Power.String(),
		Display:     r.Display.String(),
		IdleSeconds: r.Idle.Seconds(),
		ScreenLock:  string(r.Lock),
	}
}

func (f StateFile) Live(now time.Time) bool {
	if f.PollMs <= 0 || f.WrittenAt <= 0 || f.PID == nil {
		return false
	}
	poll := f.PollMs
	if now.UnixMilli()-f.WrittenAt > int64(3*poll) {
		return false
	}
	return alive(*f.PID)
}

func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func tick(ctx context.Context, host Host, paths Paths, cfg Config, state core.State) (core.State, error) {
	note := host.TakeNotification()
	reading := Snapshot(ctx, host)
	cfg.report("power", reading.PowerErr)
	cfg.report("idle", reading.IdleErr)
	cfg.report("display", reading.DisplayErr)
	cfg.report("screenlock", reading.LockErr)
	obs := core.Observation{
		Power:                 reading.Power,
		HIDIdle:               reading.Idle,
		IdleKnown:             reading.IdleErr == nil,
		SleepHeld:             host.SleepHeld(),
		NotificationDelivered: note,
	}
	next := core.Advance(core.Config{IdleThreshold: cfg.IdleThreshold}, state, obs, func(action core.Action) error {
		err := apply(ctx, host, action)
		cfg.report(action.String(), err)
		return err
	})
	pid := os.Getpid()
	st := reading.Status()
	st.Running = true
	st.PID = &pid
	st.SleepPrevented = host.SleepHeld()
	st.IdleThresholdSeconds = int(cfg.IdleThreshold / time.Second)
	st.Version = Version
	rec := StateFile{
		Status:    st,
		WrittenAt: cfg.now().UnixMilli(),
		PollMs:    int(cfg.Poll / time.Millisecond),
	}
	if err := writeState(paths, rec); err != nil {
		return next, err
	}
	return next, nil
}

func apply(ctx context.Context, host Host, action core.Action) error {
	switch action {
	case core.AcquireSleepAssertion:
		return host.AcquireSleep(ctx)
	case core.ReleaseSleepAssertion:
		return host.ReleaseSleep(ctx)
	case core.TurnDisplayOff:
		return host.DisplayOff(ctx)
	case core.DeclareActivity:
		return host.DeclareActivity(ctx)
	default:
		return fmt.Errorf("unknown action %s", action)
	}
}

func writeState(paths Paths, rec StateFile) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return WriteFileAtomic(paths.StateFile(), b, 0o644)
}

func ReadState(paths Paths) (StateFile, error) {
	b, err := os.ReadFile(paths.StateFile())
	if err != nil {
		return StateFile{}, err
	}
	var rec StateFile
	err = json.Unmarshal(b, &rec)
	return rec, err
}

func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func AppendLog(paths Paths, line string) {
	if err := os.MkdirAll(filepath.Dir(paths.LogFile()), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(paths.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintln(f, line)
}

func (c Config) wait(ctx context.Context, notes <-chan error) error {
	timer := time.NewTimer(c.Poll)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err, ok := <-notes:
		if !ok || err == nil {
			return fmt.Errorf("notifications ended")
		}
		return err
	case <-timer.C:
		return nil
	}
}

func Run(ctx context.Context, host Host, paths Paths, cfg Config) error {
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) {
			AppendLog(paths, fmt.Sprintf(format, args...))
		}
	}
	cfg.seen = map[string]string{}
	notes, err := host.StartNotifications(ctx, cfg.logf)
	if err != nil {
		return err
	}
	var state core.State
	poll := func() {
		next, err := tick(ctx, host, paths, cfg, state)
		state = next
		if err != nil {
			cfg.logf("%s", err.Error())
		}
	}
	poll()
	for {
		if err := cfg.wait(ctx, notes); err != nil {
			if relErr := host.ReleaseSleep(ctx); relErr != nil {
				cfg.logf("release: %v", relErr)
			}
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		poll()
	}
}
