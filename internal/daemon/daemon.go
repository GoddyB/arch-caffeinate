package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Wait          func(context.Context) error
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

func normalize(cfg Config) Config {
	if cfg.IdleThreshold <= 0 {
		cfg.IdleThreshold = time.Duration(DefaultIdleSeconds) * time.Second
	}
	if cfg.Poll <= 0 {
		cfg.Poll = time.Duration(DefaultPollMs) * time.Millisecond
	}
	return cfg
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
	StartNotifications(context.Context, func(string, ...any)) error
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
	SleepHeld  bool
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
	r.SleepHeld = host.SleepHeld()
	return r
}

func Tick(ctx context.Context, host Host, paths Paths, cfg Config, state core.State, pending bool) (core.State, Status, bool, error) {
	cfg = normalize(cfg)
	if host.TakeNotification() {
		pending = true
	}
	reading := Snapshot(ctx, host)
	if reading.PowerErr != nil {
		cfg.logf("power: %v", reading.PowerErr)
	}
	if reading.IdleErr != nil {
		cfg.logf("idle: %v", reading.IdleErr)
	}
	if reading.DisplayErr != nil {
		cfg.logf("display: %v", reading.DisplayErr)
	}
	if reading.LockErr != nil {
		cfg.logf("screenlock: %v", reading.LockErr)
	}
	obs := core.Observation{
		Power:                 reading.Power,
		HIDIdle:               reading.Idle,
		IdleKnown:             reading.IdleErr == nil,
		SleepHeld:             reading.SleepHeld,
		NotificationDelivered: pending,
	}
	failedDeclare := false
	next := core.Advance(core.Config{IdleThreshold: cfg.IdleThreshold}, state, obs, func(action core.Action) error {
		err := apply(ctx, host, action)
		if err != nil {
			cfg.logf("%s: %v", action, err)
			if action == core.DeclareActivity {
				failedDeclare = true
			}
		}
		return err
	})
	display := core.DisplayOn.String()
	if reading.DisplayErr == nil {
		display = reading.Display.String()
	}
	idleSeconds := 0.0
	if reading.IdleErr == nil {
		idleSeconds = reading.Idle.Seconds()
	}
	lock := string(core.ScreenLockUnknown)
	if reading.LockErr == nil {
		lock = string(reading.Lock)
	}
	pid := os.Getpid()
	st := Status{
		Running:              true,
		PID:                  &pid,
		Power:                reading.Power.String(),
		SleepPrevented:       next.SleepHeld,
		Display:              display,
		IdleSeconds:          idleSeconds,
		IdleThresholdSeconds: int(cfg.IdleThreshold / time.Second),
		ScreenLock:           lock,
		Version:              Version,
	}
	rec := StateFile{
		Status:    st,
		WrittenAt: cfg.now().UnixMilli(),
		PollMs:    int(cfg.Poll / time.Millisecond),
	}
	if err := writeState(paths, rec); err != nil {
		return next, st, failedDeclare, err
	}
	return next, st, failedDeclare, nil
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
	if err := os.MkdirAll(filepath.Dir(paths.StateFile()), 0o755); err != nil {
		return err
	}
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

func (c Config) wait(ctx context.Context) error {
	if c.Wait != nil {
		return c.Wait(ctx)
	}
	timer := time.NewTimer(c.Poll)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func Run(ctx context.Context, host Host, paths Paths, cfg Config) error {
	cfg = normalize(cfg)
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) {
			AppendLog(paths, fmt.Sprintf(format, args...))
		}
	}
	if err := host.StartNotifications(ctx, cfg.logf); err != nil {
		cfg.logf("notifications: %v", err)
	}
	var state core.State
	var pending bool
	tick := func() {
		next, _, still, err := Tick(ctx, host, paths, cfg, state, pending)
		state = next
		pending = still
		if err != nil {
			cfg.logf("%s", err.Error())
		}
	}
	tick()
	for {
		if err := cfg.wait(ctx); err != nil {
			if relErr := host.ReleaseSleep(ctx); relErr != nil {
				cfg.logf("release: %v", relErr)
			}
			return nil
		}
		tick()
	}
}
