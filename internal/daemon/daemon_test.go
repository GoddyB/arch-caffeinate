package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

type fake struct {
	power      core.Power
	powerErr   error
	idle       time.Duration
	idleErr    error
	display    core.Display
	displayErr error
	lock       core.ScreenLock
	lockErr    error
	sleep      bool
	note       bool
	actions    []core.Action
	fail       map[core.Action]int
	startErr   error
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
	if f.displayErr != nil {
		return core.DisplayOn, f.displayErr
	}
	return f.display, nil
}
func (f *fake) ScreenLock(context.Context) (core.ScreenLock, error) {
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
func (f *fake) StartNotifications(context.Context, func(string, ...any)) error {
	return f.startErr
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
	st := mustStatus(t, paths)
	if state.WakeOwed || state.Display != core.DisplayOn {
		t.Fatalf("state %#v", state)
	}
	if st.Power != "battery" || st.SleepPrevented || st.Display != "off" || st.ScreenLock != "delay:300" || st.IdleThresholdSeconds != 5 {
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
	ver, _ := got["version"].(string)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(ver) {
		t.Fatalf("version %q", ver)
	}
	delete(got, "version")
	want := map[string]any{
		"running":              true,
		"pid":                  float64(os.Getpid()),
		"power":                "battery",
		"sleepPrevented":       false,
		"display":              "off",
		"idleSeconds":          float64(2),
		"idleThresholdSeconds": float64(5),
		"screenLock":           "delay:300",
		"writtenAt":            float64(1_700_000_000_000),
		"pollMs":               float64(250),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v\nraw %s", got, want, raw)
	}
}

func TestTickDisplayFollowsLiveReading(t *testing.T) {
	home := t.TempDir()
	host := &fake{
		power:   core.PowerAC,
		idle:    time.Second,
		display: core.DisplayOff,
		lock:    core.ScreenLockOff,
		sleep:   true,
	}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: func() time.Time { return time.UnixMilli(1) }}
	_, err := tick(context.Background(), host, Paths{Home: home}, cfg, core.State{Display: core.DisplayOff})
	if err != nil {
		t.Fatal(err)
	}
	if st := mustStatus(t, Paths{Home: home}); st.Display != "off" {
		t.Fatalf("display %s", st.Display)
	}
	host.displayErr = errors.New("no panel")
	host.idleErr = errors.New("no idle")
	home2 := t.TempDir()
	_, err = tick(context.Background(), host, Paths{Home: home2}, cfg, core.State{Display: core.DisplayOff})
	if err != nil {
		t.Fatal(err)
	}
	if st := mustStatus(t, Paths{Home: home2}); st.Display != "on" {
		t.Fatalf("unknown display %s", st.Display)
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
	st := mustStatus(t, Paths{Home: home})
	if state.Display != core.DisplayOff || st.IdleSeconds != 0 || len(host.actions) != 0 {
		t.Fatalf("state %#v idle %v actions %v", state, st.IdleSeconds, host.actions)
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
	st := mustStatus(t, Paths{Home: home})
	if state.Display != core.DisplayOn || st.Power != "unknown" || st.SleepPrevented {
		t.Fatalf("state %#v status %+v", state, st)
	}
	if len(host.actions) != 1 || host.actions[0] != core.ReleaseSleepAssertion {
		t.Fatalf("actions %v", host.actions)
	}
}

func mustStatus(t *testing.T, paths Paths) Status {
	t.Helper()
	rec, err := ReadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Status
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

func TestRunLogsTickErrors(t *testing.T) {
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
	if err := Run(ctx, host, Paths{Home: home}, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Paths{Home: home}.LogFile())
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := "notifications: log stream down\npower: boom\n"
	if got != want {
		t.Fatalf("log %q", got)
	}
}
