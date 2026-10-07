package macos

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
)

var (
	idleRe    = regexp.MustCompile(`HIDIdleTime"\s*=\s*([0-9]+)`)
	delayRe   = regexp.MustCompile(`(?i)delay is\s+([0-9]+)`)
	displayRe = regexp.MustCompile(`"CurrentPowerState"\s*=\s*([0-9]+)`)
)

func ParsePower(out string) core.Power {
	line := out
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		line = out[:i]
	}
	switch {
	case strings.Contains(line, "'AC Power'"):
		return core.PowerAC
	case strings.Contains(line, "'Battery Power'"):
		return core.PowerBattery
	default:
		return core.PowerUnknown
	}
}

func ParseIdle(out string) (time.Duration, bool) {
	m := idleRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(n), true
}

func ParseScreenLock(out string) string {
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "immediate"):
		return "immediate"
	case strings.Contains(low, "off"):
		return "off"
	}
	if m := delayRe.FindStringSubmatch(out); m != nil {
		return "delay:" + m[1]
	}
	return "unknown"
}

func ParseDisplay(out string) (core.Display, bool) {
	m := displayRe.FindStringSubmatch(out)
	if m == nil {
		return core.DisplayOn, false
	}
	switch m[1] {
	case "0":
		return core.DisplayOff, true
	case "1":
		return core.DisplayOn, true
	default:
		return core.DisplayOn, false
	}
}

func ParseNotification(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "{") {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return false
	}
	msg, _ := obj["eventMessage"].(string)
	return strings.HasPrefix(msg, "Delivering ")
}
