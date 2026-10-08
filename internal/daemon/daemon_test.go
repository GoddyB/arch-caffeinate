package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
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
}

func (f *fake) Power(context.Context) (core.Power, error) {
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
func (f *fake) SleepHeld() bool { return f.sleep }
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
	return f.act(core.AcquireSleepAssertion, func() { f.sleep = true })
}
func (f *fake) ReleaseSleep(context.Context) error {
	return f.act(core.ReleaseSleepAssertion, func() { f.sleep = false })
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
func (f *fake) StartNotifications(context.Context, func(string, ...any)) (<-chan error, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	if f.noteStop != nil {
		return f.noteStop, nil
	}
	return nil, nil
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
	state, err := tick(context.Background(), host, paths, cfg, core.State{Display: core.DisplayOff})
	if err != nil {
		t.Fatal(err)
	}
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
	if _, err := tick(context.Background(), host, Paths{Home: t.TempDir()}, cfg, core.State{}); err != nil {
		t.Fatal(err)
	}
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
	state, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOn})
	if err != nil {
		t.Fatal(err)
	}
	if state.Display != core.DisplayOn {
		t.Fatalf("display %s", state.Display)
	}
	state, err = tick(context.Background(), host, Paths{Home: home}, cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	if state.Display != core.DisplayOff {
		t.Fatalf("display %s", state.Display)
	}
	want := []core.Action{core.TurnDisplayOff, core.TurnDisplayOff}
	if len(host.actions) != len(want) || host.actions[0] != want[0] || host.actions[1] != want[1] {
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
	state, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOff})
	if err != nil {
		t.Fatal(err)
	}
	if !state.WakeOwed || state.Display != core.DisplayOff {
		t.Fatalf("state %#v", state)
	}
	state, err = tick(context.Background(), host, Paths{Home: home}, cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	if state.WakeOwed || state.Display != core.DisplayOn {
		t.Fatalf("state %#v", state)
	}
	if len(host.actions) != 2 || host.actions[0] != core.DeclareActivity || host.actions[1] != core.DeclareActivity {
		t.Fatalf("actions %v", host.actions)
	}
	state, err = tick(context.Background(), host, Paths{Home: home}, cfg, state)
	if err != nil {
		t.Fatal(err)
	}
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
	state, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOff})
	if err != nil {
		t.Fatal(err)
	}
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
	state, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOn})
	if err != nil {
		t.Fatal(err)
	}
	st := mustHeartbeat(t, Paths{Home: home})
	if state.Display != core.DisplayOn || st.SleepPrevented {
		t.Fatalf("state %#v status %+v", state, st)
	}
	if len(host.actions) != 1 || host.actions[0] != core.ReleaseSleepAssertion {
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
		seen: map[string]string{},
	}
	host := &fake{powerErr: errors.New("boom"), idle: time.Second, display: core.DisplayOn, lock: core.ScreenLockOff}
	home := t.TempDir()
	if _, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{}); err != nil {
		t.Fatal(err)
	}
	if _, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{}); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "power: boom" {
		t.Fatalf("lines %v", lines)
	}
	host.powerErr = nil
	if _, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{}); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[1] != "power: cleared" {
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
	if err := Run(ctx, host, Paths{Home: home}, cfg); err != nil {
		t.Fatal(err)
	}
	if len(host.actions) != 2 || host.actions[0] != core.AcquireSleepAssertion || host.actions[1] != core.ReleaseSleepAssertion {
		t.Fatalf("actions %v", host.actions)
	}
}

func TestRunReturnsWhenNotificationsFailToStart(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		powerErr: errors.New("boom"),
		idle:     time.Second,
		display:  core.DisplayOn,
		lock:     core.ScreenLockOff,
		startErr: errors.New("log stream down"),
	}
	cfg := Config{
		IdleThreshold: time.Minute,
		Poll:          time.Hour,
		Now:           time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, host, Paths{Home: home}, cfg)
	if err == nil || err.Error() != "log stream down" {
		t.Fatalf("err %v", err)
	}
}

func TestRunReturnsWhenNotificationsEnd(t *testing.T) {
	home := t.TempDir()
	ended := make(chan error, 1)
	ended <- errors.New("log stream exited")
	host := &fake{
		power:    core.PowerAC,
		idle:     time.Second,
		display:  core.DisplayOn,
		lock:     core.ScreenLockOff,
		noteStop: ended,
	}
	cfg := Config{IdleThreshold: time.Minute, Poll: time.Hour, Now: time.Now}
	err := Run(context.Background(), host, Paths{Home: home}, cfg)
	if err == nil || err.Error() != "log stream exited" {
		t.Fatalf("err %v", err)
	}
	if len(host.actions) != 2 || host.actions[0] != core.AcquireSleepAssertion || host.actions[1] != core.ReleaseSleepAssertion {
		t.Fatalf("actions %v", host.actions)
	}
}
