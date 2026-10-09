package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

type fake struct {
	power        core.Power
	powerErr     error
	idle         time.Duration
	idleErr      error
	display      core.Display
	displayErr   error
	displayCalls int
	lock         core.ScreenLock
	lockErr      error
	lockCalls    int
	sleep        bool
	note         bool
	actions      []core.Action
	fail         map[core.Action]int
	startErr     error
	noteStop     <-chan error
	noteStarts   int
	restarted    chan struct{}
	exitNow      bool
	keepFailing  bool
	starts       []time.Time
	ticks        int
	mu           sync.Mutex
}

func (f *fake) snapshot() (starts []time.Time, ticks int, sleep bool, noteStarts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.starts...), f.ticks, f.sleep, f.noteStarts
}

type lineLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *lineLog) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *lineLog) count(want string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return countLine(l.lines, want)
}

func (f *fake) Power(context.Context) (core.Power, error) {
	f.mu.Lock()
	f.ticks++
	f.mu.Unlock()
	if f.powerErr != nil {
		return core.PowerUnknown, f.powerErr
	}
	return f.power, nil
}
func (f *fake) Idle(context.Context) (time.Duration, error) {
	if f.idleErr != nil {
		return 0, f.idleErr
	}
	return f.idle, nil
}
func (f *fake) Display(context.Context) (core.Display, error) {
	f.displayCalls++
	if f.displayErr != nil {
		return core.DisplayOn, f.displayErr
	}
	return f.display, nil
}
func (f *fake) ScreenLock(context.Context) (core.ScreenLock, error) {
	f.lockCalls++
	if f.lockErr != nil {
		return core.ScreenLockUnknown, f.lockErr
	}
	return f.lock, nil
}
func (f *fake) SleepHeld() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sleep
}
func (f *fake) TakeNotification() bool {
	v := f.note
	f.note = false
	return v
}
func (f *fake) act(a core.Action, ok func()) error {
	f.actions = append(f.actions, a)
	if f.fail[a] > 0 {
		f.fail[a]--
		return errors.New(a.String() + " failed")
	}
	if ok != nil {
		ok()
	}
	return nil
}
func (f *fake) AcquireSleep(context.Context) error {
	return f.act(core.AcquireSleepAssertion, func() {
		f.mu.Lock()
		f.sleep = true
		f.mu.Unlock()
	})
}
func (f *fake) ReleaseSleep(context.Context) error {
	return f.act(core.ReleaseSleepAssertion, func() {
		f.mu.Lock()
		f.sleep = false
		f.mu.Unlock()
	})
}
func (f *fake) DisplayOff(context.Context) error {
	return f.act(core.TurnDisplayOff, func() { f.display = core.DisplayOff })
}
func (f *fake) DeclareActivity(context.Context) error {
	return f.act(core.DeclareActivity, func() {
		f.display = core.DisplayOn
		f.idle = 0
	})
}
func (f *fake) StartNotifications(context.Context) (<-chan error, error) {
	f.mu.Lock()
	f.noteStarts++
	n := f.noteStarts
	f.starts = append(f.starts, time.Now())
	err := f.startErr
	exitNow := f.exitNow
	keep := f.keepFailing
	stop := f.noteStop
	f.mu.Unlock()
	if keep {
		return nil, err
	}
	if exitNow {
		ch := make(chan error, 1)
		ch <- errors.New("log stream exited")
		return ch, nil
	}
	if n == 1 {
		if err != nil {
			return nil, err
		}
		if stop != nil {
			return stop, nil
		}
		return nil, nil
	}
	if f.restarted != nil {
		select {
		case <-f.restarted:
		default:
			close(f.restarted)
		}
	}
	return make(chan error), nil
}

func TestTickWritesLiteralStatus(t *testing.T) {
	home := t.TempDir()
	now := time.UnixMilli(1_700_000_000_000)
	host := &fake{
		power:   core.PowerBattery,
		idle:    2 * time.Second,
		display: core.DisplayOff,
		lock:    core.ScreenLockDelay("300"),
		sleep:   true,
	}
	paths := Paths{Home: home}
	cfg := Config{
		IdleThreshold: 5 * time.Second,
		Poll:          250 * time.Millisecond,
		Now:           func() time.Time { return now },
	}
	state := tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOff}, &faults{}, nil)
	st := mustHeartbeat(t, paths)
	if state.WakeOwed || state.Display != core.DisplayOn {
		t.Fatalf("state %#v", state)
	}
	if st.SleepPrevented || st.IdleThresholdSeconds != 5 {
		t.Fatalf("%+v", st)
	}
	raw, err := os.ReadFile(paths.StateFile())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"pid":                  float64(os.Getpid()),
		"sleepPrevented":       false,
		"idleThresholdSeconds": float64(5),
		"writtenAt":            float64(1_700_000_000_000),
		"pollMs":               float64(250),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v\nraw %s", got, want, raw)
	}
}

func TestTickDoesNotReadPanelOrLock(t *testing.T) {
	host := &fake{
		power:      core.PowerAC,
		idle:       time.Second,
		displayErr: errors.New("no panel"),
		lockErr:    errors.New("no lock"),
		sleep:      true,
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: func() time.Time { return time.UnixMilli(1) }}
	tick(context.Background(), host, Paths{Home: t.TempDir()}, cfg, core.State{}, &faults{}, nil)
	if host.displayCalls != 0 || host.lockCalls != 0 {
		t.Fatalf("display %d lock %d", host.displayCalls, host.lockCalls)
	}
}

func TestTickRetriesFailedDisplayOff(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		power:   core.PowerAC,
		idle:    5 * time.Second,
		display: core.DisplayOn,
		lock:    core.ScreenLockImmediate,
		sleep:   true,
		fail:    map[core.Action]int{core.TurnDisplayOff: 1},
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: time.Now}
	state := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOn}, &faults{}, nil)
	if state.Display != core.DisplayOn {
		t.Fatalf("display %s", state.Display)
	}
	state = tick(context.Background(), host, Paths{Home: home}, cfg, state, &faults{}, nil)
	if state.Display != core.DisplayOff {
		t.Fatalf("display %s", state.Display)
	}
	want := []core.Action{core.TurnDisplayOff, core.TurnDisplayOff}
	if !slices.Equal(host.actions, want) {
		t.Fatalf("actions %v", host.actions)
	}
}

func TestTickRetriesFailedDeclare(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		power:   core.PowerAC,
		idle:    30 * time.Second,
		display: core.DisplayOff,
		lock:    core.ScreenLockOff,
		sleep:   true,
		note:    true,
		fail:    map[core.Action]int{core.DeclareActivity: 1},
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: time.Now}
	state := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOff}, &faults{}, nil)
	if !state.WakeOwed || state.Display != core.DisplayOff {
		t.Fatalf("state %#v", state)
	}
	state = tick(context.Background(), host, Paths{Home: home}, cfg, state, &faults{}, nil)
	if state.WakeOwed || state.Display != core.DisplayOn {
		t.Fatalf("state %#v", state)
	}
	if !slices.Equal(host.actions, []core.Action{core.DeclareActivity, core.DeclareActivity}) {
		t.Fatalf("actions %v", host.actions)
	}
	state = tick(context.Background(), host, Paths{Home: home}, cfg, state, &faults{}, nil)
	if state.WakeOwed || state.Display != core.DisplayOn || len(host.actions) != 2 {
		t.Fatalf("actions %v state %#v", host.actions, state)
	}
}

func TestTickIdleErrorLeavesDisplay(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		power:   core.PowerAC,
		idleErr: errors.New("no idle"),
		display: core.DisplayOff,
		lock:    core.ScreenLockOff,
		sleep:   true,
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: func() time.Time { return time.UnixMilli(1) }}
	state := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOff}, &faults{}, nil)
	if state.Display != core.DisplayOff || len(host.actions) != 0 {
		t.Fatalf("state %#v actions %v", state, host.actions)
	}
}

func TestTickPowerErrorReleases(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		powerErr: errors.New("no power"),
		idle:     time.Second,
		display:  core.DisplayOn,
		lock:     core.ScreenLockOff,
		sleep:    true,
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: time.Now}
	state := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOn}, &faults{}, nil)
	st := mustHeartbeat(t, Paths{Home: home})
	if state.Display != core.DisplayOn || st.SleepPrevented {
		t.Fatalf("state %#v status %+v", state, st)
	}
	if !slices.Equal(host.actions, []core.Action{core.ReleaseSleepAssertion}) {
		t.Fatalf("actions %v", host.actions)
	}
}

func TestReportLogsAFaultOnce(t *testing.T) {
	var lines []string
	cfg := Config{
		IdleThreshold: time.Second,
		Poll:          time.Second,
		Now:           func() time.Time { return time.UnixMilli(1) },
		Logf: func(format string, args ...any) {
			lines = append(lines, fmt.Sprintf(format, args...))
		},
	}
	fl := &faults{logf: cfg.logf}
	host := &fake{powerErr: errors.New("boom"), idle: time.Second, display: core.DisplayOn, lock: core.ScreenLockOff}
	home := t.TempDir()
	paths := Paths{Home: home}
	tick(context.Background(), host, paths, cfg, core.State{}, fl, nil)
	tick(context.Background(), host, paths, cfg, core.State{}, fl, nil)
	if len(lines) != 1 || lines[0] != "power: boom" {
		t.Fatalf("lines %v", lines)
	}
	host.powerErr = nil
	tick(context.Background(), host, paths, cfg, core.State{}, fl, nil)
	if len(lines) != 2 || lines[1] != "power: cleared" {
		t.Fatalf("lines %v", lines)
	}

	host.power = core.PowerAC
	host.sleep = false
	host.fail = map[core.Action]int{core.AcquireSleepAssertion: 5}
	before := len(lines)
	tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOn}, fl, nil)
	tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOn}, fl, nil)
	if len(lines) != before+1 || lines[before] != "AcquireSleepAssertion: AcquireSleepAssertion failed" {
		t.Fatalf("lines %v", lines)
	}
	host.power = core.PowerBattery
	host.sleep = false
	tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOn}, fl, nil)
	host.power = core.PowerAC
	tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOn}, fl, nil)
	if lines[len(lines)-1] != "AcquireSleepAssertion: AcquireSleepAssertion failed" || len(lines) != before+2 {
		t.Fatalf("lines %v", lines)
	}
}

func TestLiveRejectsNonPositivePoll(t *testing.T) {
	pid := os.Getpid()
	rec := StateFile{
		PID:       &pid,
		WrittenAt: time.Now().UnixMilli(),
		PollMs:    0,
	}
	if rec.Live(time.Now()) {
		t.Fatal("live with poll 0")
	}
}

func mustHeartbeat(t *testing.T, paths Paths) StateFile {
	t.Helper()
	rec, err := ReadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestRunReleasesOnExit(t *testing.T) {
	home := t.TempDir()
	host := &fake{power: core.PowerAC, idle: time.Second, display: core.DisplayOn, lock: core.ScreenLockOff}
	cfg := Config{
		IdleThreshold: time.Minute,
		Poll:          time.Hour,
		Now:           time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Run(ctx, host, Paths{Home: home}, cfg)
	if !slices.Equal(host.actions, []core.Action{core.AcquireSleepAssertion, core.ReleaseSleepAssertion}) {
		t.Fatalf("actions %v", host.actions)
	}
}

func countLine(lines []string, want string) int {
	n := 0
	for _, line := range lines {
		if line == want {
			n++
		}
	}
	return n
}

func TestRunKeepsSleepWhenNotificationsFailToStart(t *testing.T) {
	home := t.TempDir()
	var lines lineLog
	host := &fake{
		power:     core.PowerAC,
		idle:      time.Second,
		display:   core.DisplayOn,
		lock:      core.ScreenLockOff,
		startErr:  errors.New("log stream down"),
		restarted: make(chan struct{}),
	}
	cfg := Config{
		IdleThreshold: time.Minute,
		Poll:          time.Hour,
		NoteBackoff:   time.Millisecond,
		Now:           time.Now,
		Logf:          lines.logf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		Run(ctx, host, Paths{Home: home}, cfg)
		close(done)
	}()
	select {
	case <-host.restarted:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not restart")
	}
	_, _, sleep, starts := host.snapshot()
	if !sleep {
		t.Fatal("sleep released")
	}
	if lines.count("notifications: log stream down") != 1 {
		t.Fatal(lines.lines)
	}
	if starts < 2 {
		t.Fatalf("starts %d", starts)
	}
	cancel()
	<-done
}

func TestRunKeepsSleepWhenNotificationsEnd(t *testing.T) {
	home := t.TempDir()
	var lines lineLog
	ended := make(chan error, 1)
	ended <- errors.New("log stream exited")
	host := &fake{
		power:     core.PowerAC,
		idle:      time.Second,
		display:   core.DisplayOn,
		lock:      core.ScreenLockOff,
		noteStop:  ended,
		restarted: make(chan struct{}),
	}
	cfg := Config{
		IdleThreshold: time.Minute,
		Poll:          time.Hour,
		NoteBackoff:   time.Millisecond,
		Now:           time.Now,
		Logf:          lines.logf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		Run(ctx, host, Paths{Home: home}, cfg)
		close(done)
	}()
	select {
	case <-host.restarted:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not restart")
	}
	_, _, sleep, starts := host.snapshot()
	if !sleep {
		t.Fatal("sleep released")
	}
	if lines.count("notifications: log stream exited") != 1 {
		t.Fatal(lines.lines)
	}
	if starts < 2 {
		t.Fatalf("starts %d", starts)
	}
	cancel()
	<-done
}

func TestStreamBackoffWhilePollContinues(t *testing.T) {
	for _, mode := range []string{"exit", "start"} {
		t.Run(mode, func(t *testing.T) {
			var lines lineLog
			host := &fake{
				power:    core.PowerAC,
				idle:     time.Second,
				display:  core.DisplayOn,
				lock:     core.ScreenLockOff,
				startErr: errors.New("log stream down"),
			}
			if mode == "exit" {
				host.exitNow = true
			} else {
				host.keepFailing = true
			}
			const base = 20 * time.Millisecond
			cfg := Config{
				IdleThreshold: time.Minute,
				Poll:          base,
				NoteBackoff:   base,
				NoteCap:       4 * base,
				Now:           time.Now,
				Logf:          lines.logf,
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				Run(ctx, host, Paths{Home: t.TempDir()}, cfg)
				close(done)
			}()
			deadline := time.Now().Add(300 * time.Millisecond)
			var starts []time.Time
			var ticks int
			for time.Now().Before(deadline) {
				starts, ticks, _, _ = host.snapshot()
				if len(starts) >= 4 && ticks >= 8 {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			cancel()
			<-done
			starts, ticks, _, _ = host.snapshot()
			if ticks < 8 {
				t.Fatalf("ticks %d starts %d", ticks, len(starts))
			}
			if len(starts) < 4 {
				t.Fatalf("starts %d", len(starts))
			}
			gaps := []time.Duration{base, 2 * base, 4 * base}
			for i, want := range gaps {
				got := starts[i+1].Sub(starts[i])
				// 80ms of slack flaked on a loaded shared CI runner, where a
				// timer can stall past the whole retry schedule.
				if got < want/2 || got > want+240*time.Millisecond {
					t.Fatalf("gap %d = %s want %s starts %v", i, got, want, starts)
				}
			}
			wantLine := "notifications: log stream exited"
			if mode == "start" {
				wantLine = "notifications: log stream down"
			}
			if lines.count(wantLine) != 1 {
				t.Fatalf("lines %#v", lines.lines)
			}
		})
	}
}

func TestScratchProductionTiming(t *testing.T) {
	if os.Getenv("ARCH_BACKOFF_SCRATCH") == "" {
		t.Skip()
	}
	host := &fake{
		power:   core.PowerAC,
		idle:    time.Second,
		display: core.DisplayOn,
		lock:    core.ScreenLockOff,
		exitNow: true,
	}
	cfg := Config{
		IdleThreshold: time.Minute,
		Poll:          time.Second,
		NoteBackoff:   time.Second,
		Now:           time.Now,
		Logf:          func(string, ...any) {},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	Run(ctx, host, Paths{Home: t.TempDir()}, cfg)
	starts, ticks, _, _ := host.snapshot()
	t.Logf("ticks=%d starts=%d", ticks, len(starts))
	for i := 1; i < len(starts); i++ {
		t.Logf("gap %d = %s", i, starts[i].Sub(starts[i-1]))
	}
}
