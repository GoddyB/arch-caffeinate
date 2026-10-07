#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$ROOT/artifacts/verify/$STAMP"
mkdir -p "$OUT"
TRANSCRIPT="$OUT/transcript.txt"
: >"$TRANSCRIPT"

FAILS=0

line() {
  printf '%s\n' "$1" | tee -a "$TRANSCRIPT"
}

capture() {
  local dest="$1"
  shift
  {
    printf '$ %s\n' "$*"
    "$@"
    printf '\nexit:%s\n' "$?"
  } >"$dest" 2>&1 || true
  cat "$dest" >>"$TRANSCRIPT"
}

mark() {
  local verdict="$1"
  local id="$2"
  local reason="$3"
  if [[ "$verdict" == PASS ]]; then
    line "PASS $id $reason"
  else
    line "FAIL $id $reason"
    FAILS=$((FAILS + 1))
  fi
}

finish() {
  line "evidence $STAMP"
  if [[ "$FAILS" -eq 0 ]]; then
    exit 0
  fi
  exit 1
}

not_macos() {
  line "FAIL doctor not macOS"
  mark FAIL V1 "not macOS"
  mark FAIL V2 "not macOS"
  mark FAIL V3 "not macOS"
  mark FAIL V4 "not macOS"
  mark FAIL V5 "not macOS"
  mark FAIL V6 "not macOS"
  mark FAIL V7 "not macOS"
  finish
}

if [[ "$(uname -s)" != Darwin ]]; then
  not_macos
fi

LABEL="io.github.goddyb.arch-caffeinate"
PLIST="${HOME}/Library/LaunchAgents/${LABEL}.plist"
BIN="$(bash -lc 'command -v arch-caffeinate' 2>>"$TRANSCRIPT" || true)"

missing_cli() {
  line "FAIL doctor arch-caffeinate is not on PATH in a login bash"
  mark FAIL V1 "arch-caffeinate is not on PATH"
  mark FAIL V2 "arch-caffeinate is not on PATH"
  mark FAIL V3 "arch-caffeinate is not on PATH"
  mark FAIL V4 "arch-caffeinate is not on PATH"
  mark FAIL V5 "arch-caffeinate is not on PATH"
  mark FAIL V6 "arch-caffeinate is not on PATH"
  mark FAIL V7 "arch-caffeinate is not on PATH"
  finish
}

if [[ -z "$BIN" ]]; then
  missing_cli
fi

json_get() {
  python3 - "$1" "$2" <<'PY'
import json, sys
path, key = sys.argv[1], sys.argv[2]
with open(path) as f:
    data = json.load(f)
if key not in data or data[key] is None:
    sys.exit(2)
val = data[key]
if isinstance(val, bool):
    print("true" if val else "false")
else:
    print(val)
PY
}

require_fields() {
  python3 - "$1" <<'PY'
import json, sys
need = [
    "running", "pid", "power", "sleepPrevented", "display",
    "idleSeconds", "idleThresholdSeconds", "screenLock", "version",
]
with open(sys.argv[1]) as f:
    data = json.load(f)
missing = [k for k in need if k not in data]
if missing:
    print("missing " + ",".join(missing))
    sys.exit(1)
print("ok")
PY
}

wait_until() {
  local seconds="$1"
  local i=0
  shift
  while [[ "$i" -lt "$seconds" ]]; do
    if "$@"; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

display_reading() {
  python3 - "$1" <<'PY'
import re, sys
text = open(sys.argv[1], errors="replace").read()
m = re.search(r'"CurrentPowerState"\s*=\s*(\d+)', text)
if not m:
    sys.exit(1)
print(m.group(1))
PY
}

display_is() {
  local want="$1"
  local want_n=1
  if [[ "$want" == "off" ]]; then
    want_n=0
  fi
  capture "$OUT/display-ioreg.txt" ioreg -r -d 1 -c AppleCLCD2
  local got reported
  got="$(display_reading "$OUT/display-ioreg.txt" 2>>"$TRANSCRIPT" || true)"
  arch-caffeinate status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || return 1
  reported="$(json_get "$OUT/status.json" display 2>>"$TRANSCRIPT" || true)"
  [[ "$got" == "$want_n" && "$reported" == "$want" ]]
}

"$BIN" install --idle-seconds 5 >>"$TRANSCRIPT" 2>&1 || true
"$BIN" install --idle-seconds 5 >>"$TRANSCRIPT" 2>&1 || true

doctor_out="$("$BIN" doctor 2>&1)" || true
doctor_code=$?
printf '%s\n' "$doctor_out" >>"$TRANSCRIPT"
printf 'doctor_exit:%s\n' "$doctor_code" >>"$TRANSCRIPT"
if [[ "$doctor_code" -eq 0 ]]; then
  line "PASS doctor exit 0"
else
  line "FAIL doctor exit ${doctor_code}"
  FAILS=$((FAILS + 1))
fi

"$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || true
power="$(json_get "$OUT/status.json" power 2>>"$TRANSCRIPT" || echo unknown)"

capture "$OUT/assertions.txt" pmset -g assertions
pid="$(json_get "$OUT/status.json" pid 2>>"$TRANSCRIPT" || echo none)"
if [[ "$power" == "ac" ]] && grep -q PreventSystemSleep "$OUT/assertions.txt" && grep -q "$pid" "$OUT/assertions.txt"; then
  mark PASS V1 "PreventSystemSleep lists pid ${pid} while power is ac"
else
  mark FAIL V1 "power is ${power} or PreventSystemSleep does not list the daemon pid"
fi

capture "$OUT/hid.txt" ioreg -c IOHIDSystem
if wait_until 8 display_is off; then
  mark PASS V2 "AppleCLCD2 CurrentPowerState is 0 and status display is off"
else
  mark FAIL V2 "display stayed on for 8 seconds at idle threshold 5"
fi

line "simulated user activity: arch-caffeinate wake"
"$BIN" wake >>"$TRANSCRIPT" 2>&1 || true
wake_input=0
if wait_until 3 display_is on; then
  wake_input=1
fi

if wait_until 8 display_is off; then
  osascript -e 'display notification "verify" with title "arch-caffeinate"' >>"$TRANSCRIPT" 2>&1 || true
  wake_note=0
  if wait_until 3 display_is on; then
    wake_note=1
  fi
else
  wake_note=0
fi

if [[ "$wake_input" -eq 1 && "$wake_note" -eq 1 ]]; then
  mark PASS V3 "AppleCLCD2 CurrentPowerState is 1 after simulated user activity and after a notification"
else
  mark FAIL V3 "input wake=${wake_input} notification wake=${wake_note}"
fi

capture "$OUT/screenlock.txt" sysadminctl -screenLock status
lock="$(json_get "$OUT/status.json" screenLock 2>>"$TRANSCRIPT" || echo missing)"
if [[ "$lock" == "off" ]] && grep -qi 'off' "$OUT/screenlock.txt"; then
  mark PASS V4 "sysadminctl and status screenLock are off"
else
  mark FAIL V4 "screenLock is ${lock}"
fi

if [[ -n "$BIN" ]]; then
  mark PASS V5 "login bash resolves arch-caffeinate"
else
  mark FAIL V5 "login bash does not resolve arch-caffeinate"
fi

GHOSTTY=""
if command -v ghostty >/dev/null 2>&1; then
  GHOSTTY="$(command -v ghostty)"
elif [[ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ]]; then
  GHOSTTY="/Applications/Ghostty.app/Contents/MacOS/ghostty"
fi
if [[ -n "$GHOSTTY" ]]; then
  capture "$OUT/ghostty-config.txt" "$GHOSTTY" +show-config
  if grep -E 'command *= *.*/?bash([[:space:]]|$)' "$OUT/ghostty-config.txt" >/dev/null 2>&1; then
    mark PASS V6 "Ghostty command is bash"
  else
    mark FAIL V6 "Ghostty command is not bash"
  fi
else
  mark FAIL V6 "Ghostty is not installed"
fi

if require_fields "$OUT/status.json" >>"$TRANSCRIPT" 2>&1 \
  && grep -q 'idle-seconds' "$PLIST" \
  && launchctl print "gui/${UID}/${LABEL}" >"$OUT/launchctl.txt" 2>>"$TRANSCRIPT"; then
  mark PASS V7 "install loaded the agent and status --json has the contract fields"
else
  mark FAIL V7 "install, launchctl print, or status fields failed"
fi

"$BIN" install >>"$TRANSCRIPT" 2>&1 || true
"$BIN" status --json >"$OUT/status-after-cleanup.json" 2>>"$TRANSCRIPT" || true
restored="$(json_get "$OUT/status-after-cleanup.json" idleThresholdSeconds 2>>"$TRANSCRIPT" || echo missing)"
if [[ "$restored" == "600" ]]; then
  line "cleanup idleThresholdSeconds is 600"
else
  line "FAIL cleanup idleThresholdSeconds is ${restored}"
  FAILS=$((FAILS + 1))
fi

finish
