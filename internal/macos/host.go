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

type System struct {
	mu      sync.Mutex
	pending bool
	sleep   *exec.Cmd
	execCmd func(context.Context, string, ...string) *exec.Cmd
}

func (s *System) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if s.execCmd != nil {
		return s.execCmd(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...)
}

func (s *System) Power(ctx context.Context) (core.Power, error) {
	out, err := s.output(ctx, "pmset", "-g", "ps")
	if err != nil {
		return core.PowerUnknown, err
	}
	return ParsePower(out), nil
}

func (s *System) Idle(ctx context.Context) (time.Duration, error) {
	out, err := s.output(ctx, "ioreg", "-c", "IOHIDSystem")
	if err != nil {
		return 0, err
	}
	d, ok := ParseIdle(out)
	if !ok {
		return 0, errMissing("HIDIdleTime")
	}
	return d, nil
}

func (s *System) ScreenLock(ctx context.Context) (core.ScreenLock, error) {
	out, err := s.combined(ctx, "sysadminctl", "-screenLock", "status")
	lock := ParseScreenLock(out)
	if lock == core.ScreenLockUnknown && err != nil {
		return core.ScreenLockUnknown, err
	}
	return lock, nil
}

func (s *System) Display(ctx context.Context) (core.Display, error) {
	out, err := s.output(ctx, "ioreg", "-r", "-d", "1", "-c", "AppleCLCD2")
	if err != nil {
		return core.DisplayOn, err
	}
	d, ok := ParseDisplay(out)
	if !ok {
		return core.DisplayOn, errMissing("CurrentPowerState")
	}
	return d, nil
}

func (s *System) SleepHeld() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sleep != nil && s.sleep.Process != nil
}

func (s *System) TakeNotification() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.pending
	s.pending = false
	return v
}

func (s *System) AcquireSleep(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sleep != nil && s.sleep.Process != nil {
		return nil
	}
	cmd := s.command(ctx, "caffeinate", "-s", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return err
	}
	s.sleep = cmd
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if s.sleep == cmd {
			s.sleep = nil
		}
		s.mu.Unlock()
	}()
	return nil
}

func (s *System) ReleaseSleep(context.Context) error {
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
	return s.command(ctx, "pmset", "displaysleepnow").Run()
}

func (s *System) DeclareActivity(ctx context.Context) error {
	cmd := s.command(ctx, "caffeinate", "-u", "-t", "1")
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (s *System) StartNotifications(ctx context.Context, logf func(string, ...any)) error {
	cmd := s.command(ctx, "/usr/bin/log", "stream", "--style", "ndjson", "--predicate",
		`process == "usernoted" AND subsystem == "com.apple.unc" AND eventMessage BEGINSWITH "Delivering "`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if ParseNotification(sc.Text()) {
				s.mu.Lock()
				s.pending = true
				s.mu.Unlock()
			}
		}
		err := cmd.Wait()
		if ctx.Err() == nil && logf != nil {
			logf("log stream exited: %v", err)
		}
	}()
	return nil
}

type missing string

func (m missing) Error() string { return string(m) }

func errMissing(name string) error { return missing(name + " missing") }

func (s *System) output(ctx context.Context, name string, args ...string) (string, error) {
	b, err := s.command(ctx, name, args...).Output()
	return string(b), err
}

func (s *System) combined(ctx context.Context, name string, args ...string) (string, error) {
	b, err := s.command(ctx, name, args...).CombinedOutput()
	return string(b), err
}
