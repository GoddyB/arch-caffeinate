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
	idleRe       = regexp.MustCompile(`HIDIdleTime"\s*=\s*([0-9]+)`)
	displayRe    = regexp.MustCompile(`"CurrentPowerState"\s*=\s*([0-9]+)`)
	screenLockRe = regexp.MustCompile(`(?i)screenLock delay is\s+(immediate|off|([0-9]+))`)
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

func ParseScreenLock(out string) core.ScreenLock {
	matches := screenLockRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return core.ScreenLockUnknown
	}
	m := matches[len(matches)-1]
	switch strings.ToLower(m[1]) {
	case "immediate":
		return core.ScreenLockImmediate
	case "off":
		return core.ScreenLockOff
	default:
		return core.ScreenLockDelay(m[2])
	}
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
