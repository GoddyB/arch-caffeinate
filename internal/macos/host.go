package macos

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

type Host interface {
	Power(ctx context.Context) (core.Power, error)
	Idle(ctx context.Context) (time.Duration, error)
	ScreenLock(ctx context.Context) (string, error)
	Display(ctx context.Context) (core.Display, bool, error)
	NotificationPending() bool
	ClearNotification()
	AcquireSleep(ctx context.Context) error
	ReleaseSleep() error
	DisplayOff(ctx context.Context) error
	DeclareActivity(ctx context.Context) error
	StartNotifications(ctx context.Context) error
}

type System struct {
	mu      sync.Mutex
	pending bool
	sleep   *exec.Cmd
	logCmd  *exec.Cmd
}

func (s *System) Power(ctx context.Context) (core.Power, error) {
	out, err := output(ctx, "pmset", "-g", "ps")
	if err != nil {
		return core.PowerUnknown, err
	}
	return ParsePower(out), nil
}

func (s *System) Idle(ctx context.Context) (time.Duration, error) {
	out, err := output(ctx, "ioreg", "-c", "IOHIDSystem")
	if err != nil {
		return 0, err
	}
	d, ok := ParseIdle(out)
	if !ok {
		return 0, errMissing("HIDIdleTime")
	}
	return d, nil
}

func (s *System) ScreenLock(ctx context.Context) (string, error) {
	out, err := combined(ctx, "sysadminctl", "-screenLock", "status")
	if err != nil && out == "" {
		return "unknown", err
	}
	return ParseScreenLock(out), nil
}

func (s *System) Display(ctx context.Context) (core.Display, bool, error) {
	out, err := output(ctx, "ioreg", "-r", "-d", "1", "-c", "AppleCLCD2")
	if err != nil {
		return core.DisplayOn, false, err
	}
	d, ok := ParseDisplay(out)
	return d, ok, nil
}

func (s *System) NotificationPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

func (s *System) ClearNotification() {
	s.mu.Lock()
	s.pending = false
	s.mu.Unlock()
}

func (s *System) AcquireSleep(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sleep != nil && s.sleep.Process != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "caffeinate", "-s", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return err
	}
	s.sleep = cmd
	go func() { _ = cmd.Wait() }()
	return nil
}

func (s *System) ReleaseSleep() error {
	s.mu.Lock()
	cmd := s.sleep
	s.sleep = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Kill()
	return nil
}

func (s *System) DisplayOff(ctx context.Context) error {
	return exec.CommandContext(ctx, "pmset", "displaysleepnow").Run()
}

func (s *System) DeclareActivity(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "caffeinate", "-u", "-t", "1")
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (s *System) StartNotifications(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/log", "stream", "--style", "ndjson", "--predicate",
		`process == "usernoted" AND subsystem == "com.apple.unc" AND eventMessage BEGINSWITH "Delivering "`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.logCmd = cmd
	s.mu.Unlock()
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if ParseNotification(sc.Text()) {
				s.mu.Lock()
				s.pending = true
				s.mu.Unlock()
			}
		}
		_ = cmd.Wait()
	}()
	return nil
}

type missing string

func (m missing) Error() string { return string(m) }

func errMissing(name string) error { return missing(name + " missing") }

func output(ctx context.Context, name string, args ...string) (string, error) {
	b, err := exec.CommandContext(ctx, name, args...).Output()
	return string(b), err
}

func combined(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}
