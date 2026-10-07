package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

type fake struct {
	power   core.Power
	idle    time.Duration
	display core.Display
	hasDisp bool
	lock    string
	note    bool
	actions []core.Action
}

func (f *fake) Power(context.Context) (core.Power, error) { return f.power, nil }
func (f *fake) Idle(context.Context) (time.Duration, error) {
	return f.idle, nil
}
func (f *fake) ScreenLock(context.Context) (string, error) { return f.lock, nil }
func (f *fake) Display(context.Context) (core.Display, bool, error) {
	return f.display, f.hasDisp, nil
}
func (f *fake) NotificationPending() bool { return f.note }
func (f *fake) ClearNotification()        { f.note = false }
func (f *fake) AcquireSleep(context.Context) error {
	f.actions = append(f.actions, core.AcquireSleepAssertion)
	return nil
}
func (f *fake) ReleaseSleep() error {
	f.actions = append(f.actions, core.ReleaseSleepAssertion)
	return nil
}
func (f *fake) DisplayOff(context.Context) error {
	f.actions = append(f.actions, core.TurnDisplayOff)
	f.display = core.DisplayOff
	f.hasDisp = true
	return nil
}
func (f *fake) DeclareActivity(context.Context) error {
	f.actions = append(f.actions, core.DeclareActivity)
	f.display = core.DisplayOn
	f.hasDisp = true
	return nil
}
func (f *fake) StartNotifications(context.Context) error { return nil }

func TestTickWritesLiteralStatus(t *testing.T) {
	home := t.TempDir()
	now := time.UnixMilli(1_700_000_000_000)
	host := &fake{
		power:   core.PowerAC,
		idle:    2 * time.Second,
		display: core.DisplayOn,
		hasDisp: true,
		lock:    "off",
	}
	paths := Paths{Home: home}
	cfg := Config{
		IdleThreshold: 600 * time.Second,
		Poll:          time.Second,
		Now:           func() time.Time { return now },
	}
	_, st, err := Tick(context.Background(), host, paths, cfg, core.State{})
	if err != nil {
		t.Fatal(err)
	}
	if !st.SleepPrevented || st.Power != "ac" || st.Display != "on" || st.ScreenLock != "off" {
		t.Fatalf("%+v", st)
	}
	if len(host.actions) != 1 || host.actions[0] != core.AcquireSleepAssertion {
		t.Fatalf("actions %v", host.actions)
	}
	got, err := ReadStatus(paths)
	if err != nil {
		t.Fatal(err)
	}
	if got.WrittenAt != now.UnixMilli() || got.IdleThresholdSeconds != 600 || got.Version != Version {
		t.Fatalf("%+v", got)
	}
	if got.IdleSeconds != 2 {
		t.Fatalf("idle %v", got.IdleSeconds)
	}
}

func TestTickDisplayOffOnce(t *testing.T) {
	home := t.TempDir()
	host := &fake{power: core.PowerAC, idle: 5 * time.Second, display: core.DisplayOn, hasDisp: true, lock: "immediate"}
	cfg := Config{IdleThreshold: 5 * time.Second, Poll: time.Second, Now: time.Now}
	state, _, err := Tick(context.Background(), host, Paths{Home: home}, cfg, core.State{SleepHeld: true, Display: core.DisplayOn})
	if err != nil {
		t.Fatal(err)
	}
	if state.Display != core.DisplayOff {
		t.Fatalf("display %s", state.Display)
	}
	_, _, err = Tick(context.Background(), host, Paths{Home: home}, cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	offs := 0
	for _, a := range host.actions {
		if a == core.TurnDisplayOff {
			offs++
		}
	}
	if offs != 1 {
		t.Fatalf("DisplayOff count %d actions %v", offs, host.actions)
	}
}
