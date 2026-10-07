package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
	"github.com/GoddyB/arch-caffeinate/internal/macos"
)

const Version = "0.1.0"

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
	WrittenAt            int64   `json:"writtenAt,omitempty"`
	PollMs               int     `json:"pollMs,omitempty"`
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
	return filepath.Join(p.Home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
}

func (p Paths) Bin() string {
	return filepath.Join(p.Home, ".local", "bin", "arch-caffeinate")
}

type Config struct {
	IdleThreshold time.Duration
	Poll          time.Duration
	Now           func() time.Time
	Logf          func(string, ...any)
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

func Tick(ctx context.Context, host macos.Host, paths Paths, cfg Config, state core.State) (core.State, Status, error) {
	now := cfg.now()
	power, powerErr := host.Power(ctx)
	if powerErr != nil {
		cfg.logf("power: %v", powerErr)
		power = core.PowerUnknown
	}
	idle, idleErr := host.Idle(ctx)
	if idleErr != nil {
		cfg.logf("idle: %v", idleErr)
		if state.Display == core.DisplayOff {
			idle = cfg.coreConfig().IdleThreshold
		} else {
			idle = 0
		}
	}
	note := host.NotificationPending()
	obs := core.Observation{
		Now:                   now,
		Power:                 power,
		HIDIdle:               idle,
		NotificationDelivered: note,
	}
	if idleErr != nil {
		obs.HIDIdle = 0
		obs.NotificationDelivered = note
	}
	next, actions := core.Step(cfg.coreConfig(), state, obs)
	failed := map[core.Action]bool{}
	for _, action := range actions {
		if err := apply(ctx, host, action); err != nil {
			cfg.logf("%s: %v", action, err)
			failed[action] = true
		}
	}
	if failed[core.AcquireSleepAssertion] || failed[core.ReleaseSleepAssertion] {
		next.SleepHeld = state.SleepHeld
	}
	if failed[core.TurnDisplayOff] || failed[core.DeclareActivity] {
		next.Display = state.Display
	}
	if note && !failed[core.DeclareActivity] {
		host.ClearNotification()
	}

	display := next.Display.String()
	if live, ok, err := host.Display(ctx); err == nil && ok {
		display = live.String()
	}
	lock := "unknown"
	if s, err := host.ScreenLock(ctx); err == nil && s != "" {
		lock = s
	}
	pid := os.Getpid()
	st := Status{
		Running:              true,
		PID:                  &pid,
		Power:                power.String(),
		SleepPrevented:       next.SleepHeld,
		Display:              display,
		IdleSeconds:          idle.Seconds(),
		IdleThresholdSeconds: int(cfg.IdleThreshold / time.Second),
		ScreenLock:           lock,
		Version:              Version,
		WrittenAt:            now.UnixMilli(),
		PollMs:               int(cfg.Poll / time.Millisecond),
	}
	if err := writeStatus(paths, st); err != nil {
		return next, st, err
	}
	return next, st, nil
}

func (c Config) coreConfig() core.Config {
	threshold := c.IdleThreshold
	if threshold <= 0 {
		threshold = 600 * time.Second
	}
	return core.Config{IdleThreshold: threshold}
}

func apply(ctx context.Context, host macos.Host, action core.Action) error {
	switch action {
	case core.AcquireSleepAssertion:
		return host.AcquireSleep(ctx)
	case core.ReleaseSleepAssertion:
		return host.ReleaseSleep()
	case core.TurnDisplayOff:
		return host.DisplayOff(ctx)
	case core.DeclareActivity:
		return host.DeclareActivity(ctx)
	default:
		return fmt.Errorf("unknown action %s", action)
	}
}

func writeStatus(paths Paths, st Status) error {
	if err := os.MkdirAll(filepath.Dir(paths.StateFile()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := paths.StateFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, paths.StateFile())
}

func ReadStatus(paths Paths) (Status, error) {
	b, err := os.ReadFile(paths.StateFile())
	if err != nil {
		return Status{}, err
	}
	var st Status
	err = json.Unmarshal(b, &st)
	return st, err
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

func Run(ctx context.Context, host macos.Host, paths Paths, cfg Config) error {
	if cfg.Poll <= 0 {
		cfg.Poll = time.Second
	}
	if cfg.IdleThreshold <= 0 {
		cfg.IdleThreshold = 600 * time.Second
	}
	_ = host.StartNotifications(ctx)
	var state core.State
	tick := func() {
		next, _, err := Tick(ctx, host, paths, cfg, state)
		state = next
		if err != nil {
			AppendLog(paths, err.Error())
		}
	}
	tick()
	ticker := time.NewTicker(cfg.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = host.ReleaseSleep()
			return nil
		case <-ticker.C:
			tick()
		}
	}
}
