package core

import "time"

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

type Config struct {
	IdleThreshold time.Duration
}

type Observation struct {
	Now                   time.Time
	Power                 Power
	HIDIdle               time.Duration
	NotificationDelivered bool
}

type State struct {
	SleepHeld bool
	Display   Display
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
		return "DisplayOff"
	case DeclareActivity:
		return "DeclareActivity"
	default:
		return "unknown"
	}
}

func Step(cfg Config, s State, o Observation) (State, []Action) {
	next := s
	var actions []Action

	if o.Power == PowerAC && !s.SleepHeld {
		next.SleepHeld = true
		actions = append(actions, AcquireSleepAssertion)
	}
	if o.Power != PowerAC && s.SleepHeld {
		next.SleepHeld = false
		actions = append(actions, ReleaseSleepAssertion)
	}

	idleHigh := o.HIDIdle >= cfg.IdleThreshold
	if o.NotificationDelivered || !idleHigh {
		if o.NotificationDelivered {
			actions = append(actions, DeclareActivity)
		}
		next.Display = DisplayOn
		return next, actions
	}
	if s.Display == DisplayOn {
		next.Display = DisplayOff
		actions = append(actions, TurnDisplayOff)
	}
	return next, actions
}
