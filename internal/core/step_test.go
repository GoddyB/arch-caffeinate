package core

import (
	"testing"
	"time"
)

func TestStep(t *testing.T) {
	cfg := Config{IdleThreshold: 600 * time.Second}
	now := time.Unix(1_700_000_000, 0)
	onAC := State{SleepHeld: true, Display: DisplayOn}
	offAC := State{SleepHeld: false, Display: DisplayOn}
	asleep := State{SleepHeld: true, Display: DisplayOff}

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
			o:    Observation{Now: now, Power: PowerBattery, HIDIdle: time.Second},
			want: offAC,
			acts: []Action{ReleaseSleepAssertion},
		},
		{
			name: "battery to ac acquires the assertion",
			s:    offAC,
			o:    Observation{Now: now, Power: PowerAC, HIDIdle: time.Second},
			want: onAC,
			acts: []Action{AcquireSleepAssertion},
		},
		{
			name: "idle crossing the threshold turns the display off once",
			s:    onAC,
			o:    Observation{Now: now, Power: PowerAC, HIDIdle: 600 * time.Second},
			want: State{SleepHeld: true, Display: DisplayOff},
			acts: []Action{TurnDisplayOff},
		},
		{
			name: "staying idle does not repeat DisplayOff",
			s:    asleep,
			o:    Observation{Now: now, Power: PowerAC, HIDIdle: 900 * time.Second},
			want: asleep,
			acts: nil,
		},
		{
			name: "activity after off turns the display on",
			s:    asleep,
			o:    Observation{Now: now, Power: PowerAC, HIDIdle: 10 * time.Second},
			want: onAC,
			acts: nil,
		},
		{
			name: "notification after off declares activity and turns the display on",
			s:    asleep,
			o:    Observation{Now: now, Power: PowerAC, HIDIdle: 900 * time.Second, NotificationDelivered: true},
			want: onAC,
			acts: []Action{DeclareActivity},
		},
		{
			name: "unknown power releases a held assertion",
			s:    onAC,
			o:    Observation{Now: now, Power: PowerUnknown, HIDIdle: time.Second},
			want: offAC,
			acts: []Action{ReleaseSleepAssertion},
		},
		{
			name: "unknown power does not acquire",
			s:    offAC,
			o:    Observation{Now: now, Power: PowerUnknown, HIDIdle: time.Second},
			want: offAC,
			acts: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, acts := Step(cfg, tc.s, tc.o)
			if got != tc.want {
				t.Fatalf("state %#v, want %#v", got, tc.want)
			}
			if len(acts) != len(tc.acts) {
				t.Fatalf("actions %v, want %v", acts, tc.acts)
			}
			for i := range acts {
				if acts[i] != tc.acts[i] {
					t.Fatalf("actions %v, want %v", acts, tc.acts)
				}
			}
		})
	}
}
