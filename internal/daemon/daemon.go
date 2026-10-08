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
	PID                  *int  `json:"pid"`
	SleepPrevented       bool  `json:"sleepPrevented"`
	IdleThresholdSeconds int   `json:"idleThresholdSeconds"`
	WrittenAt            int64 `json:"writtenAt"`
	PollMs               int   `json:"pollMs"`
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
	NoteBackoff   time.Duration
}

type faults struct {
	prev map[string]string
	logf func(string, ...any)
}

func (f *faults) report(name string, err error) {
	if f == nil {
		return
	}
	if f.prev == nil {
		f.prev = map[string]string{}
	}
	if err == nil {
		if _, ok := f.prev[name]; ok {
			delete(f.prev, name)
			if f.logf != nil {
				f.logf("%s: cleared", name)
			}
		}
		return
	}
	msg := err.Error()
	if f.prev[name] == msg {
		return
	}
	f.prev[name] = msg
	if f.logf != nil {
		f.logf("%s: %v", name, err)
	}
}

func (f *faults) drop(name string) {
	if f == nil || f.prev == nil {
		return
	}
	delete(f.prev, name)
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
	StartNotifications(context.Context) (<-chan error, error)
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
		Version:     Version,
	}
}

func (f StateFile) Live(now time.Time) bool {
	if f.PollMs <= 0 || f.WrittenAt <= 0 || f.PID == nil {
		return false
	}
	if now.UnixMilli()-f.WrittenAt > int64(3*f.PollMs) {
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

func tick(ctx context.Context, host Host, paths Paths, cfg Config, state core.State, fl *faults) (core.State, error) {
	note := host.TakeNotification()
	power, powerErr := host.Power(ctx)
	if powerErr != nil {
		power = core.PowerUnknown
	}
	idle, idleErr := host.Idle(ctx)
	if idleErr != nil {
		idle = 0
	}
	fl.report("power", powerErr)
	fl.report("idle", idleErr)
	obs := core.Observation{
		Power:                 power,
		HIDIdle:               idle,
		IdleKnown:             idleErr == nil,
		SleepHeld:             host.SleepHeld(),
		NotificationDelivered: note,
	}
	emitted := map[core.Action]bool{}
	next := core.Advance(core.Config{IdleThreshold: cfg.IdleThreshold}, state, obs, func(action core.Action) error {
		emitted[action] = true
		err := apply(ctx, host, action)
		fl.report(action.String(), err)
		return err
	})
	for _, action := range []core.Action{core.AcquireSleepAssertion, core.ReleaseSleepAssertion, core.TurnDisplayOff, core.DeclareActivity} {
		if !emitted[action] {
			fl.drop(action.String())
		}
	}
	pid := os.Getpid()
	rec := StateFile{
		PID:                  &pid,
		SleepPrevented:       host.SleepHeld(),
		IdleThresholdSeconds: int(cfg.IdleThreshold / time.Second),
		WrittenAt:            cfg.now().UnixMilli(),
		PollMs:               int(cfg.Poll / time.Millisecond),
	}
	fl.report("state", writeState(paths, rec))
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

func (c Config) noteBase() time.Duration {
	if c.NoteBackoff > 0 {
		return c.NoteBackoff
	}
	return time.Second
}

func Run(ctx context.Context, host Host, paths Paths, cfg Config) error {
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) {
			AppendLog(paths, fmt.Sprintf(format, args...))
		}
	}
	fl := &faults{logf: cfg.logf}
	base := cfg.noteBase()
	capDelay := 60 * time.Second
	var notes <-chan error
	var delay time.Duration
	waiting := false
	arm := func() {
		if delay == 0 {
			delay = base
		} else {
			delay *= 2
		}
		if delay > capDelay {
			delay = capDelay
		}
		waiting = true
	}
	openStream := func() {
		if notes != nil || waiting {
			return
		}
		ch, err := host.StartNotifications(ctx)
		if err != nil {
			fl.report("notifications", err)
			arm()
			return
		}
		notes = ch
		delay = 0
		fl.report("notifications", nil)
	}
	var state core.State
	step := func() {
		state, _ = tick(ctx, host, paths, cfg, state, fl)
	}
	openStream()
	step()
	for {
		var retry <-chan time.Time
		var retryTimer *time.Timer
		if waiting {
			retryTimer = time.NewTimer(delay)
			retry = retryTimer.C
		}
		pollTimer := time.NewTimer(cfg.Poll)
		select {
		case <-ctx.Done():
			pollTimer.Stop()
			if retryTimer != nil {
				retryTimer.Stop()
			}
			if relErr := host.ReleaseSleep(ctx); relErr != nil {
				cfg.logf("release: %v", relErr)
			}
			return nil
		case err, ok := <-notes:
			pollTimer.Stop()
			if retryTimer != nil {
				retryTimer.Stop()
			}
			notes = nil
			if !ok || err == nil {
				err = fmt.Errorf("log stream exited")
			}
			fl.report("notifications", err)
			arm()
		case <-pollTimer.C:
			if retryTimer != nil {
				retryTimer.Stop()
			}
			step()
		case <-retry:
			pollTimer.Stop()
			waiting = false
			openStream()
		}
	}
}
