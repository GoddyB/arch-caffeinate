#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${VERIFY_OUT:-$ROOT/artifacts/verify/$STAMP}"
mkdir -p "$OUT"
TRANSCRIPT="$OUT/transcript.txt"
: >"$TRANSCRIPT"

FAILS=0
POLL_SEC="${VERIFY_POLL_SEC:-1}"
INSTALL_TEST="${VERIFY_INSTALL_TEST:-$ROOT/scripts/test-install-from-clone.sh}"

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
  mark FAIL V8 "not macOS"
  finish
}

if [[ "$(uname -s)" != Darwin ]]; then
  not_macos
fi

LABEL="io.github.goddyb.arch-caffeinate"
STATE="${HOME}/Library/Application Support/arch-caffeinate/state.json"
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
  mark FAIL V8 "arch-caffeinate is not on PATH"
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

wait_until() {
  local seconds="$1"
  local i=0
  shift
  while [[ "$i" -lt "$seconds" ]]; do
    if "$@"; then
      return 0
    fi
    sleep "$POLL_SEC"
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
  "$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || return 1
  reported="$(json_get "$OUT/status.json" display 2>>"$TRANSCRIPT" || true)"
  [[ "$got" == "$want_n" && "$reported" == "$want" ]]
}

hold_display() {
  local want="$1"
  local samples="$2"
  local i=0
  while [[ "$i" -lt "$samples" ]]; do
    display_is "$want" || return 1
    sleep "$POLL_SEC"
    i=$((i + 1))
  done
  return 0
}

hid_ns() {
  python3 - "$1" <<'PY'
import re, sys
text = open(sys.argv[1], errors="replace").read()
m = re.search(r'HIDIdleTime"\s*=\s*([0-9]+)', text)
if not m:
    sys.exit(1)
print(m.group(1))
PY
}

idle_and_off() {
  capture "$OUT/hid.txt" ioreg -c IOHIDSystem
  local ns
  ns="$(hid_ns "$OUT/hid.txt" 2>>"$TRANSCRIPT" || true)"
  [[ -n "$ns" && "$ns" -ge 5000000000 ]] && display_is off
}

running_is() {
  "$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || return 1
  [[ "$(json_get "$OUT/status.json" running 2>>"$TRANSCRIPT" || true)" == "true" ]]
}

screen_lock_phrase() {
  python3 - "$1" <<'PY'
import re, sys
text = open(sys.argv[1], errors="replace").read()
matches = re.findall(r'(?i)screenLock delay is\s+(immediate|off|[0-9]+)', text)
if not matches:
    sys.exit(1)
print(matches[-1].lower())
PY
}

state_is_fresh() {
  python3 - "$STATE" "1000" "3" <<'PY'
import json, sys, time
path, poll_ms, polls = sys.argv[1], float(sys.argv[2]), float(sys.argv[3])
try:
    data = json.load(open(path))
except (OSError, json.JSONDecodeError):
    sys.exit(1)
written = data.get("writtenAt")
if not isinstance(written, (int, float)) or isinstance(written, bool):
    sys.exit(1)
if abs(time.time() * 1000 - float(written)) <= poll_ms * polls:
    sys.exit(0)
sys.exit(1)
PY
}

install_ok=0
if "$BIN" install --idle-seconds 5 >>"$TRANSCRIPT" 2>&1; then
  cp "$PLIST" "$OUT/plist.1"
  if "$BIN" install --idle-seconds 5 >>"$TRANSCRIPT" 2>&1; then
    if cmp -s "$PLIST" "$OUT/plist.1"; then
      install_ok=1
    fi
  fi
fi

state_is_fresh || true
if wait_until 3 state_is_fresh; then
  line "PASS heartbeat writtenAt is fresh"
else
  line "FAIL heartbeat writtenAt is not fresh"
  FAILS=$((FAILS + 1))
fi

doctor_out="$("$BIN" doctor 2>&1)"
doctor_code=$?
printf '%s\n' "$doctor_out" >>"$TRANSCRIPT"
printf 'doctor_exit:%s\n' "$doctor_code" >>"$TRANSCRIPT"
doctor_lines_ok=1
if [[ -z "$doctor_out" ]]; then
  doctor_lines_ok=0
else
  while IFS= read -r doctor_line; do
    [[ -z "$doctor_line" ]] && continue
    if [[ ! "$doctor_line" =~ ^(PASS|WARN)([[:space:]]|$) ]]; then
      doctor_lines_ok=0
    fi
  done <<<"$doctor_out"
fi
if [[ "$doctor_code" -eq 0 && "$doctor_lines_ok" -eq 1 ]]; then
  line "PASS doctor exit 0"
else
  line "FAIL doctor exit ${doctor_code}"
  FAILS=$((FAILS + 1))
fi

"$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || true
power="$(json_get "$OUT/status.json" power 2>>"$TRANSCRIPT" || echo unknown)"
prevented="$(json_get "$OUT/status.json" sleepPrevented 2>>"$TRANSCRIPT" || echo missing)"

capture "$OUT/assertions.txt" pmset -g assertions
pid="$(json_get "$OUT/status.json" pid 2>>"$TRANSCRIPT" || echo none)"
child="$(pgrep -P "$pid" -x caffeinate 2>>"$TRANSCRIPT" | head -n 1 || true)"
if [[ "$power" == "ac" && "$prevented" == "true" && -n "$child" ]] \
  && grep -Eq '^ *PreventSystemSleep +[1-9][0-9]*$' "$OUT/assertions.txt" \
  && grep -Eq "pid ${child}\\(caffeinate\\):" "$OUT/assertions.txt"; then
  mark PASS V1 "PreventSystemSleep lists pid ${child}(caffeinate) while power is ac and sleepPrevented is true"
elif [[ "$power" == "battery" ]]; then
  mark FAIL V1 "power is battery; releasing the assertion on battery is a manual gap"
else
  mark FAIL V1 "power is ${power}, sleepPrevented is ${prevented}, or PreventSystemSleep does not list pid ${child}(caffeinate)"
fi

if wait_until 8 idle_and_off; then
  mark PASS V2 "HIDIdleTime is at least 5s and AppleCLCD2 CurrentPowerState is 0 and status display is off"
else
  mark FAIL V2 "display stayed on for 8 seconds or HIDIdleTime stayed under 5s"
fi

line "wake coverage: arch-caffeinate wake stands in for mouse or key input"
wake_input=0
hid_reset=0
reoff=0
if display_is off; then
  capture "$OUT/hid-before-wake.txt" ioreg -c IOHIDSystem
  before_ns="$(hid_ns "$OUT/hid-before-wake.txt" 2>>"$TRANSCRIPT" || true)"
  "$BIN" wake >>"$TRANSCRIPT" 2>&1 || true
  capture "$OUT/hid-after-wake.txt" ioreg -c IOHIDSystem
  after_ns="$(hid_ns "$OUT/hid-after-wake.txt" 2>>"$TRANSCRIPT" || true)"
  if [[ -n "$before_ns" && -n "$after_ns" && "$after_ns" -lt "$before_ns" ]] && hold_display on 3; then
    wake_input=1
    hid_reset=1
  fi
fi
if [[ "$wake_input" -eq 1 ]] && wait_until 8 idle_and_off; then
  reoff=1
fi

"$BIN" stop >>"$TRANSCRIPT" 2>&1 || true
pmset displaysleepnow >>"$TRANSCRIPT" 2>&1 || true
panel_off=0
if wait_until 3 display_is off; then
  panel_off=1
fi
osascript -e 'display notification "verify" with title "arch-caffeinate"' >>"$TRANSCRIPT" 2>&1 || true
note_while_stopped=0
if [[ "$reoff" -eq 1 && "$panel_off" -eq 1 ]] && hold_display off 3; then
  note_while_stopped=1
fi

old_pid="$(json_get "$STATE" pid 2>>"$TRANSCRIPT" || echo none)"
"$BIN" start >>"$TRANSCRIPT" 2>&1 || true
fresh_daemon() {
  local now_pid
  now_pid="$(json_get "$STATE" pid 2>>"$TRANSCRIPT" || true)"
  [[ -n "$now_pid" && "$now_pid" != "$old_pid" ]] && state_is_fresh
}
note_while_started=0
if wait_until 8 running_is && fresh_daemon; then
  osascript -e 'display notification "verify" with title "arch-caffeinate"' >>"$TRANSCRIPT" 2>&1 || true
  if wait_until 3 display_is on; then
    note_while_started=1
  fi
fi

if [[ "$wake_input" -eq 1 && "$hid_reset" -eq 1 && "$reoff" -eq 1 && "$note_while_stopped" -eq 1 && "$note_while_started" -eq 1 ]]; then
  mark PASS V3 "wake stands in for mouse or key input and HIDIdleTime drops; the display is off again before stop; pmset displaysleepnow leaves it off while stopped; notification turns it on after start"
else
  mark FAIL V3 "wake=${wake_input} hid_reset=${hid_reset} reoff=${reoff} panel_off=${panel_off} notification_while_stopped=${note_while_stopped} notification_after_start=${note_while_started}"
fi

capture "$OUT/screenlock.txt" sysadminctl -screenLock status
"$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || true
lock="$(json_get "$OUT/status.json" screenLock 2>>"$TRANSCRIPT" || echo missing)"
phrase="$(screen_lock_phrase "$OUT/screenlock.txt" 2>>"$TRANSCRIPT" || true)"
quoted="$(grep -E -i 'screenLock delay is' "$OUT/screenlock.txt" | tail -n 1 || true)"
if [[ -z "$quoted" ]]; then
  quoted="$(grep -v -E '^(exit:|\$ )' "$OUT/screenlock.txt" | tr '\n' ' ' | sed 's/[[:space:]]*$//' || true)"
fi
if [[ -z "$quoted" ]]; then
  quoted="missing"
fi
if [[ "$lock" == "off" && "$phrase" == "off" ]] && grep -q 'PASS screenlock: off' <<<"$doctor_out"; then
  mark PASS V4 "sysadminctl phrase is off, status screenLock is off, and doctor prints PASS screenlock: off"
else
  mark FAIL V4 "screenLock is ${lock}; sysadminctl: ${quoted}"
fi

resolved_version="$(bash -lc 'arch-caffeinate --version' 2>>"$TRANSCRIPT" || true)"
bin_version="$("$BIN" --version 2>>"$TRANSCRIPT" || true)"
if [[ "$BIN" == "$HOME/.local/bin/arch-caffeinate" && -n "$bin_version" && "$resolved_version" == "$bin_version" ]]; then
  mark PASS V5 "login bash runs $HOME/.local/bin/arch-caffeinate and --version matches"
else
  mark FAIL V5 "login bash resolved ${BIN:-empty}"
fi

GHOSTTY=""
if command -v ghostty >/dev/null 2>&1; then
  GHOSTTY="$(command -v ghostty)"
elif [[ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ]]; then
  GHOSTTY="/Applications/Ghostty.app/Contents/MacOS/ghostty"
fi
if [[ -n "$GHOSTTY" ]]; then
  capture "$OUT/ghostty-config.txt" "$GHOSTTY" +show-config
  if grep -Eq '^command = (/bin/|/opt/homebrew/bin/)?bash( .*)?$' "$OUT/ghostty-config.txt"; then
    mark PASS V6 "Ghostty command = line runs bash"
  else
    mark FAIL V6 "Ghostty command = line does not run bash"
  fi
else
  mark FAIL V6 "Ghostty is not installed"
fi

"$BIN" status --json >"$OUT/status.json" 2>>"$TRANSCRIPT" || true
running="$(json_get "$OUT/status.json" running 2>>"$TRANSCRIPT" || echo missing)"
status_pid="$(json_get "$OUT/status.json" pid 2>>"$TRANSCRIPT" || echo missing)"
threshold="$(json_get "$OUT/status.json" idleThresholdSeconds 2>>"$TRANSCRIPT" || echo missing)"
display_field="$(json_get "$OUT/status.json" display 2>>"$TRANSCRIPT" || echo missing)"
status_version="$(json_get "$OUT/status.json" version 2>>"$TRANSCRIPT" || echo missing)"
bin_version_one="$("$BIN" --version 2>>"$TRANSCRIPT" | tr -d '\r\n' || true)"
launch_ok=0
launch_pid=""
if launchctl print "gui/${UID}/${LABEL}" >"$OUT/launchctl.txt" 2>>"$TRANSCRIPT"; then
  launch_ok=1
  launch_pid="$(sed -n 's/^pid = \([0-9][0-9]*\)$/\1/p' "$OUT/launchctl.txt" | head -n 1)"
fi
power_ok=0
case "$power" in
  ac|battery|unknown) power_ok=1 ;;
esac
display_ok=0
case "$display_field" in
  on|off) display_ok=1 ;;
esac
if [[ "$install_ok" -eq 1 && "$launch_ok" -eq 1 && "$running" == "true" \
  && "$status_pid" == "$launch_pid" && -n "$launch_pid" \
  && "$threshold" == "5" && "$power_ok" -eq 1 && "$display_ok" -eq 1 \
  && "$status_version" == "$bin_version_one" && -n "$bin_version_one" ]]; then
  mark PASS V7 "second install matches the first plist, launchctl pid matches status, idleThresholdSeconds is 5"
else
  mark FAIL V7 "install_ok=${install_ok} running=${running} pid=${status_pid} launch_pid=${launch_pid} threshold=${threshold} power=${power} display=${display_field} version=${status_version}"
fi

v8_out="$OUT/v8.txt"
if [[ -x "$INSTALL_TEST" ]] && "$INSTALL_TEST" >"$v8_out" 2>&1; then
  cat "$v8_out" >>"$TRANSCRIPT"
  mark PASS V8 "$INSTALL_TEST exited 0"
else
  cat "$v8_out" >>"$TRANSCRIPT" 2>/dev/null || true
  mark FAIL V8 "$INSTALL_TEST failed"
fi

"$BIN" install >>"$TRANSCRIPT" 2>&1 || true
threshold_is_600() {
  "$BIN" status --json >"$OUT/status-after-cleanup.json" 2>>"$TRANSCRIPT" || return 1
  [[ "$(json_get "$OUT/status-after-cleanup.json" idleThresholdSeconds 2>>"$TRANSCRIPT" || true)" == "600" ]]
}
restored_ok=0
threshold_is_600 || true
if wait_until 3 threshold_is_600; then
  restored_ok=1
fi
if [[ "$restored_ok" -eq 1 ]] && [[ -f "$PLIST" ]] && ! grep -q -- '--idle-seconds' "$PLIST"; then
  line "cleanup idleThresholdSeconds is 600"
else
  restored="$(json_get "$OUT/status-after-cleanup.json" idleThresholdSeconds 2>>"$TRANSCRIPT" || echo missing)"
  line "FAIL cleanup idleThresholdSeconds is ${restored}"
  FAILS=$((FAILS + 1))
fi

finish
