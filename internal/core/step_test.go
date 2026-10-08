package core

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestAdvance(t *testing.T) {
	cfg := Config{IdleThreshold: 600 * time.Second}
	onAC := State{Display: DisplayOn}
	offAC := State{Display: DisplayOn}
	asleep := State{Display: DisplayOff}
	open := Observation{IdleKnown: true, SleepHeld: false, Power: PowerAC}

	cases := []struct {
		name string
		s    State
		o    Observation
		want State
		acts []Action
	}{
		{
			name: "ac to battery releases the assertion",
			s:    onAC,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerBattery, HIDIdle: time.Second},
			want: offAC,
			acts: []Action{ReleaseSleepAssertion},
		},
		{
			name: "battery to ac acquires the assertion",
			s:    offAC,
			o:    open,
			want: onAC,
			acts: []Action{AcquireSleepAssertion},
		},
		{
			name: "idle crossing the threshold turns the display off once",
			s:    onAC,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 600 * time.Second},
			want: State{Display: DisplayOff},
			acts: []Action{TurnDisplayOff},
		},
		{
			name: "staying idle does not repeat TurnDisplayOff",
			s:    asleep,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 900 * time.Second},
			want: asleep,
		},
		{
			name: "activity after off turns the display on",
			s:    asleep,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 10 * time.Second},
			want: onAC,
		},
		{
			name: "notification after off declares activity and turns the display on",
			s:    asleep,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 900 * time.Second, NotificationDelivered: true},
			want: onAC,
			acts: []Action{DeclareActivity},
		},
		{
			name: "notification during high idle keeps the display on",
			s:    onAC,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 900 * time.Second, NotificationDelivered: true},
			want: onAC,
			acts: []Action{DeclareActivity},
		},
		{
			name: "battery to ac and idle crossing emit acquire then display off",
			s:    State{Display: DisplayOn},
			o:    Observation{IdleKnown: true, Power: PowerAC, HIDIdle: 600 * time.Second},
			want: State{Display: DisplayOff},
			acts: []Action{AcquireSleepAssertion, TurnDisplayOff},
		},
		{
			name: "unknown power releases a held assertion",
			s:    onAC,
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerUnknown, HIDIdle: time.Second},
			want: offAC,
			acts: []Action{ReleaseSleepAssertion},
		},
		{
			name: "unknown power does not acquire",
			s:    State{Display: DisplayOff},
			o:    Observation{IdleKnown: true, Power: PowerUnknown, HIDIdle: 900 * time.Second},
			want: State{Display: DisplayOff},
		},
		{
			name: "unknown idle leaves the display unchanged",
			s:    asleep,
			o:    Observation{SleepHeld: true, Power: PowerAC},
			want: asleep,
		},
		{
			name: "notification with unknown idle is declared",
			s:    asleep,
			o:    Observation{SleepHeld: true, Power: PowerAC, NotificationDelivered: true},
			want: onAC,
			acts: []Action{DeclareActivity},
		},
		{
			name: "a failed declare stays owed until it succeeds",
			s:    State{Display: DisplayOff, WakeOwed: true},
			o:    Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 900 * time.Second},
			want: onAC,
			acts: []Action{DeclareActivity},
		},
		{
			name: "display turns off on battery",
			s:    State{Display: DisplayOn},
			o:    Observation{IdleKnown: true, Power: PowerBattery, HIDIdle: 600 * time.Second},
			want: State{Display: DisplayOff},
			acts: []Action{TurnDisplayOff},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []Action
			state := Advance(cfg, tc.s, tc.o, func(a Action) error {
				got = append(got, a)
				return nil
			})
			if state != tc.want {
				t.Fatalf("state %#v, want %#v", state, tc.want)
			}
			if !slices.Equal(got, tc.acts) {
				t.Fatalf("actions %v, want %v", got, tc.acts)
			}
		})
	}
}

func TestAdvanceKeepsStateWhenActionFails(t *testing.T) {
	cfg := Config{IdleThreshold: 5 * time.Second}
	start := State{Display: DisplayOn}
	obs := Observation{IdleKnown: true, SleepHeld: true, Power: PowerAC, HIDIdle: 5 * time.Second}
	boom := errors.New("display off failed")
	var calls []Action
	state := Advance(cfg, start, obs, func(a Action) error {
		calls = append(calls, a)
		return boom
	})
	if state != start {
		t.Fatalf("state %#v", state)
	}
	if !slices.Equal(calls, []Action{TurnDisplayOff}) {
		t.Fatalf("actions %v", calls)
	}
	state = Advance(cfg, state, obs, func(a Action) error {
		calls = append(calls, a)
		return nil
	})
	if state != (State{Display: DisplayOff}) {
		t.Fatalf("state %#v", state)
	}
	if !slices.Equal(calls, []Action{TurnDisplayOff, TurnDisplayOff}) {
		t.Fatalf("actions %v", calls)
	}
}
