package core

import (
	"fmt"
	"time"
)

type Power int

const (
	PowerUnknown Power = iota
	PowerAC
	PowerBattery
)

func (p Power) String() string {
	switch p {
	case PowerAC:
		return "ac"
	case PowerBattery:
		return "battery"
	default:
		return "unknown"
	}
}

type Display int

const (
	DisplayOn Display = iota
	DisplayOff
)

func (d Display) String() string {
	if d == DisplayOff {
		return "off"
	}
	return "on"
}

type ScreenLock string

const (
	ScreenLockOff       ScreenLock = "off"
	ScreenLockImmediate ScreenLock = "immediate"
	ScreenLockUnknown   ScreenLock = "unknown"
)

func ScreenLockDelay(seconds string) ScreenLock {
	return ScreenLock("delay:" + seconds)
}

type Config struct {
	IdleThreshold time.Duration
}

type Observation struct {
	Power                 Power
	HIDIdle               time.Duration
	IdleKnown             bool
	SleepHeld             bool
	NotificationDelivered bool
}

type State struct {
	Display  Display
	WakeOwed bool
}

type Action int

const (
	AcquireSleepAssertion Action = iota
	ReleaseSleepAssertion
	TurnDisplayOff
	DeclareActivity
)

func (a Action) String() string {
	switch a {
	case AcquireSleepAssertion:
		return "AcquireSleepAssertion"
	case ReleaseSleepAssertion:
		return "ReleaseSleepAssertion"
	case TurnDisplayOff:
		return "TurnDisplayOff"
	case DeclareActivity:
		return "DeclareActivity"
	default:
		return fmt.Sprintf("action(%d)", int(a))
	}
}

func (s State) After(a Action) State {
	switch a {
	case TurnDisplayOff:
		s.Display = DisplayOff
	case DeclareActivity:
		s.Display = DisplayOn
		s.WakeOwed = false
	}
	return s
}

func Step(cfg Config, s State, o Observation) (State, []Action) {
	next := s
	var actions []Action
	switch {
	case o.Power == PowerAC && !o.SleepHeld:
		actions = append(actions, AcquireSleepAssertion)
	case o.Power != PowerAC && o.SleepHeld:
		actions = append(actions, ReleaseSleepAssertion)
	}
	next.WakeOwed = s.WakeOwed || o.NotificationDelivered
	if next.WakeOwed {
		actions = append(actions, DeclareActivity)
	}
	if o.IdleKnown {
		switch {
		case o.HIDIdle < cfg.IdleThreshold:
			next.Display = DisplayOn
		case s.Display == DisplayOn && !next.WakeOwed:
			actions = append(actions, TurnDisplayOff)
		}
	}
	return next, actions
}

func Advance(cfg Config, s State, o Observation, do func(Action) error) State {
	next, actions := Step(cfg, s, o)
	for _, action := range actions {
		if err := do(action); err != nil {
			continue
		}
		next = next.After(action)
	}
	return next
}
