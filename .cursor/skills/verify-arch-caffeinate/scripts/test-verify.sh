#!/usr/bin/env bash
set -u

SKILL_DIR="$(cd "$(dirname "$0")" && pwd)"
VERIFY="$SKILL_DIR/verify.sh"

fail() {
  printf 'test-verify: %s\n' "$1" >&2
  exit 1
}

write_stubs() {
  local bin="$1"
  mkdir -p "$bin"

  cat >"$bin/uname" <<'EOF'
#!/bin/sh
printf '%s\n' Darwin
EOF

  cat >"$bin/pmset" <<'EOF'
#!/bin/sh
if [ "$1" = "displaysleepnow" ]; then
  printf '%s\n' 0 >"$VERIFY_FIXTURE/display"
  exit 0
fi
cat "$VERIFY_FIXTURE/assertions.txt"
EOF

  cat >"$bin/ioreg" <<'EOF'
#!/bin/sh
case "$*" in
  *AppleCLCD2*)
    printf '"CurrentPowerState"=%s\n' "$(cat "$VERIFY_FIXTURE/display")"
    ;;
  *IOHIDSystem*)
    if [ "$VERIFY_MODE" = "broken" ]; then
      printf '%s\n' '"HIDIdleTime" = 0'
      exit 0
    fi
    ns=$(cat "$VERIFY_FIXTURE/idle_ns")
    ns=$((ns + 1000000000))
    printf '%s\n' "$ns" >"$VERIFY_FIXTURE/idle_ns"
    if [ "$ns" -ge 5000000000 ]; then
      printf '%s\n' 0 >"$VERIFY_FIXTURE/display"
    fi
    printf '"HIDIdleTime" = %s\n' "$ns"
    ;;
esac
EOF

  cat >"$bin/sysadminctl" <<'EOF'
#!/bin/sh
cat "$VERIFY_FIXTURE/screenlock.txt" >&2
EOF

  cat >"$bin/launchctl" <<'EOF'
#!/bin/bash
if [[ "${VERIFY_MODE}" == "broken" ]]; then
  printf '%s\n' 'launchctl refused' >&2
  exit 1
fi
mode="stopped"
if [[ -f "$VERIFY_FIXTURE/agent_mode" ]]; then
  mode="$(cat "$VERIFY_FIXTURE/agent_mode")"
fi
if [[ "$mode" == "stopped" ]]; then
  printf '%s\n' 'Print failed: 3: No such process' >&2
  exit 1
fi
pid_file="$VERIFY_FIXTURE/parent_pid"
if [ -f "$VERIFY_FIXTURE/daemon_pid" ]; then
  pid_file="$VERIFY_FIXTURE/daemon_pid"
fi
printf 'pid = %s\n' "$(cat "$pid_file")"
exit 0
EOF

  cat >"$bin/ghostty" <<'EOF'
#!/bin/sh
cat "$HOME/.config/ghostty/config"
EOF

  cat >"$bin/osascript" <<'EOF'
#!/bin/bash
if [[ "$VERIFY_MODE" == "healthy" ]]; then
  mode="$(cat "$VERIFY_FIXTURE/agent_mode")"
  if [[ "$mode" == "started" ]]; then
    printf '%s\n' 1 >"$VERIFY_FIXTURE/display"
  fi
fi
EOF

  cat >"$bin/arch-caffeinate" <<'EOF'
#!/bin/bash
set -u
fix="$VERIFY_FIXTURE"
cmd="${1:-}"
shift || true
plist="${HOME}/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist"
state_dir="${HOME}/Library/Application Support/arch-caffeinate"
mkdir -p "${HOME}/.local/bin" "$(dirname "$plist")" "$state_dir"

write_state() {
  python3 - "$fix" "$state_dir/state.json" "$VERIFY_MODE" <<'PY'
import json, os, sys, time
fix, dest, mode = sys.argv[1:]
cleanup = os.path.exists(os.path.join(fix, "cleanup"))
agent_path = os.path.join(fix, "agent_mode")
agent = open(agent_path).read().strip() if os.path.exists(agent_path) else ""
pid_path = os.path.join(fix, "daemon_pid")
if not os.path.exists(pid_path):
    pid_path = os.path.join(fix, "parent_pid")
if mode == "healthy":
    running = agent == "started"
    payload = {
        "pid": int(open(pid_path).read()) if running else None,
        "sleepPrevented": True,
        "idleThresholdSeconds": 600 if cleanup else 5,
        "writtenAt": int(time.time() * 1000) if running else 0,
        "pollMs": 1000,
    }
else:
    payload = {
        "pid": None,
        "sleepPrevented": False,
        "idleThresholdSeconds": 5,
        "writtenAt": 0,
        "pollMs": 1000,
    }
with open(dest, "w") as f:
    json.dump(payload, f)
PY
}

print_status() {
  python3 - "$fix" "$state_dir/state.json" "$VERIFY_MODE" <<'PY'
import json, os, sys
fix, path, mode = sys.argv[1:]
display = open(os.path.join(fix, "display")).read().strip()
saved = json.load(open(path))
written = saved.get("writtenAt") or 0
poll = saved.get("pollMs") or 0
live = mode == "healthy" and isinstance(written, (int, float)) and written > 0 and poll > 0
print(json.dumps({
    "running": live,
    "pid": saved.get("pid") if live else None,
    "power": "ac",
    "sleepPrevented": bool(saved.get("sleepPrevented")) if live else False,
    "display": "on" if display == "1" else "off",
    "idleSeconds": 6 if mode == "healthy" else 0,
    "idleThresholdSeconds": saved.get("idleThresholdSeconds"),
    "screenLock": "off" if mode == "healthy" else "immediate",
    "version": "1.0.0",
}))
PY
}

case "$cmd" in
  install)
    n=0
    if [[ -f "$fix/install_count" ]]; then
      n="$(cat "$fix/install_count")"
    fi
    n=$((n + 1))
    printf '%s\n' "$n" >"$fix/install_count"
    body='<string>run</string>'
    if [[ " $* " == *" --idle-seconds "* ]] || [[ -f "$fix/force_idle_flag" ]]; then
      body='<string>run</string>
<string>--idle-seconds</string>
<string>5</string>'
    fi
    if [[ "$VERIFY_MODE" == "healthy" && " $* " != *" --idle-seconds "* ]]; then
      : >"$fix/cleanup"
    fi
    if [[ "$VERIFY_MODE" == "broken" ]]; then
      : >"$fix/force_idle_flag"
    fi
    printf '%s\n' "$body" >"$plist"
    if [[ "$VERIFY_MODE" == "broken" && "$n" -ge 2 && " $* " == *" --idle-seconds "* ]]; then
      printf '%s\n' drift >>"$plist"
    fi
    cp "$0" "${HOME}/.local/bin/arch-caffeinate"
    write_state
    ;;
  doctor)
    if [[ "$VERIFY_MODE" == "broken" ]]; then
      printf '%s\n' 'FAIL macOS: stub'
      exit 1
    fi
    if [[ -f "$fix/doctor.txt" ]]; then
      cat "$fix/doctor.txt"
      exit 0
    fi
    printf '%s\n' 'PASS macOS' 'PASS heartbeat: fresh' 'PASS screenlock: off'
    ;;
  status)
    write_state
    print_status
    ;;
  wake)
    if [[ "$VERIFY_MODE" == "healthy" ]]; then
      printf '%s\n' 1 >"$fix/display"
      printf '%s\n' 0 >"$fix/idle_ns"
    fi
    ;;
  startup)
    sub="${1:-}"
    case "$sub" in
      install)
        if [[ ! -f "$plist" ]]; then
          printf '%s\n' 'plist missing, run install first' >&2
          exit 1
        fi
        if [[ "$VERIFY_MODE" == "healthy" ]]; then
          loads=0
          if [[ -f "$fix/loads" ]]; then
            loads="$(cat "$fix/loads")"
          fi
          loads=$((loads + 1))
          printf '%s\n' "$loads" >"$fix/loads"
          parent="$(cat "$fix/parent_pid")"
          printf '%s\n' "$((parent + loads - 1))" >"$fix/daemon_pid"
          printf '%s\n' started >"$fix/agent_mode"
          write_state
        fi
        ;;
      remove)
        if [[ "$VERIFY_MODE" == "healthy" ]]; then
          printf '%s\n' stopped >"$fix/agent_mode"
        fi
        ;;
      *)
        printf '%s\n' 'usage: arch-caffeinate startup <install|remove>' >&2
        exit 2
        ;;
    esac
    ;;
  --version)
    printf '%s\n' '1.0.0'
    ;;
  *)
    printf 'unknown %s\n' "$cmd" >&2
    exit 2
    ;;
esac
EOF

  chmod +x "$bin/uname" "$bin/pmset" "$bin/ioreg" "$bin/sysadminctl" \
    "$bin/launchctl" "$bin/ghostty" "$bin/osascript" "$bin/arch-caffeinate"
}

prepare() {
  local mode="$1"
  local work="$2"
  local home="$work/home"
  local bin="$work/bin"
  local fix="$work/fixture"
  mkdir -p "$home/.config/ghostty" "$home/.local/bin" "$fix"
  write_stubs "$bin"
  cp "$bin/arch-caffeinate" "$home/.local/bin/arch-caffeinate"
  if [[ "$mode" == "healthy" ]]; then
    printf '%s\n' "export PATH=\"\$HOME/.local/bin:\$PATH\"" >"$home/.bash_profile"
    printf '%s\n' 1 >"$fix/display"
    printf '%s\n' 0 >"$fix/idle_ns"
    printf '%s\n' stopped >"$fix/agent_mode"
    printf '%s\n' 'screenLock delay is off' >"$fix/screenlock.txt"
    printf '%s\n' 'command = /bin/bash' >"$home/.config/ghostty/config"
    cp "$(command -v sleep)" "$bin/caffeinate"
    cat >"$work/parent.sh" <<EOF
#!/bin/sh
"$bin/caffeinate" 30 &
wait
EOF
    chmod +x "$work/parent.sh"
    "$work/parent.sh" >/dev/null 2>&1 &
    echo $! >"$fix/parent_pid"
    local child=""
    local i=0
    while [[ "$i" -lt 50 ]]; do
      child="$(pgrep -P "$(cat "$fix/parent_pid")" -x caffeinate | head -n 1 || true)"
      if [[ -n "$child" ]]; then
        break
      fi
      i=$((i + 1))
    done
    [[ -n "$child" ]] || fail "caffeinate child did not start"
    printf '   PreventSystemSleep  2\n   pid %s(caffeinate): PreventSystemSleep named: "arch-caffeinate"\n' "$child" >"$fix/assertions.txt"
  else
    mkdir -p "$home/other/bin"
    cp "$bin/arch-caffeinate" "$home/other/bin/arch-caffeinate"
    printf '%s\n' "export PATH=\"\$HOME/other/bin:\$PATH\"" >"$home/.bash_profile"
    printf '%s\n' 1 >"$fix/display"
    printf '%s\n' 0 >"$fix/idle_ns"
    printf '%s\n' stopped >"$fix/agent_mode"
    printf '%s\n' 'screenLock delay is immediate' >"$fix/screenlock.txt"
    printf '%s\n' 'command = /bin/zsh' 'initial-command = bash' >"$home/.config/ghostty/config"
    printf '%s\n' 1 >"$fix/parent_pid"
    printf '%s\n' '   PreventSystemSleep  0' >"$fix/assertions.txt"
  fi
  printf '%s\n' '#!/bin/sh' 'exit '"$3" >"$work/install-test.sh"
  chmod +x "$work/install-test.sh"
}

assert_fail_lines() {
  local log="$1"
  local id
  for id in V1 V2 V3 V4 V5 V6 V7 V8; do
    grep -q "^FAIL ${id} " "$log" || fail "missing FAIL ${id} in broken run"
  done
}

assert_only_v4_fails() {
  local log="$1"
  local label="$2"
  local id
  grep -q "^FAIL V4 " "$log" || fail "${label} missing FAIL V4"
  for id in V1 V2 V3 V5 V6 V7 V8; do
    grep -q "^PASS ${id} " "$log" || fail "${label} missing PASS ${id}"
    if grep -q "^FAIL ${id} " "$log"; then
      fail "${label} unexpected FAIL ${id}"
    fi
  done
}

drive() {
  local work="$1"
  local mode="$2"
  local log="$3"
  HOME="$work/home" \
    VERIFY_MODE="$mode" \
    VERIFY_FIXTURE="$work/fixture" \
    VERIFY_POLL_SEC=0 \
    VERIFY_OUT="$work/out" \
    VERIFY_INSTALL_TEST="$work/install-test.sh" \
    PATH="$work/bin:$PATH" \
    "$VERIFY" >"$log" 2>&1
}

stop_parent() {
  local work="$1"
  if [[ -f "$work/fixture/parent_pid" ]]; then
    local parent
    parent="$(cat "$work/fixture/parent_pid")"
    pkill -P "$parent" 2>/dev/null || true
    kill "$parent" 2>/dev/null || true
  fi
}
cleanup_runs() {
  stop_parent "${HOME_RUN:-}"
  stop_parent "${BROKEN_RUN:-}"
  stop_parent "${BANNER_RUN:-}"
  stop_parent "${DOCTOR_RUN:-}"
  rm -rf "${HOME_RUN:-}" "${BROKEN_RUN:-}" "${BANNER_RUN:-}" "${DOCTOR_RUN:-}"
  rm -f "${healthy_log:-}" "${broken_log:-}" "${banner_log:-}" "${doctor_log:-}"
}
trap cleanup_runs EXIT

set +e
healthy_log="$(mktemp)"
broken_log="$(mktemp)"

HOME_RUN="$(mktemp -d)"
prepare healthy "$HOME_RUN" 0
drive "$HOME_RUN" healthy "$healthy_log"
healthy_code=$?

BROKEN_RUN="$(mktemp -d)"
prepare broken "$BROKEN_RUN" 1
drive "$BROKEN_RUN" broken "$broken_log"
broken_code=$?

banner_log="$(mktemp)"
BANNER_RUN="$(mktemp -d)"
prepare healthy "$BANNER_RUN" 0
printf '%s\n' 'usage: turn screen lock off in Settings' >"$BANNER_RUN/fixture/screenlock.txt"
drive "$BANNER_RUN" healthy "$banner_log"
banner_code=$?

doctor_log="$(mktemp)"
DOCTOR_RUN="$(mktemp -d)"
prepare healthy "$DOCTOR_RUN" 0
printf '%s\n' 'PASS macOS' 'PASS heartbeat: fresh' >"$DOCTOR_RUN/fixture/doctor.txt"
drive "$DOCTOR_RUN" healthy "$doctor_log"
doctor_code=$?
set -e

stop_parent "$HOME_RUN"
stop_parent "$BANNER_RUN"
stop_parent "$DOCTOR_RUN"

printf '%s\n' "--- healthy exit ${healthy_code} ---"
cat "$healthy_log"
printf '%s\n' "--- broken exit ${broken_code} ---"
cat "$broken_log"
printf '%s\n' "--- usage banner exit ${banner_code} ---"
cat "$banner_log"
printf '%s\n' "--- doctor omission exit ${doctor_code} ---"
cat "$doctor_log"

[[ "$healthy_code" -eq 0 ]] || fail "healthy stub exited ${healthy_code}"
grep -q '^FAIL ' "$healthy_log" && fail "healthy stub printed FAIL"
for id in V1 V2 V3 V4 V5 V6 V7 V8; do
  grep -q "^PASS ${id} " "$healthy_log" || fail "missing PASS ${id}"
done

[[ "$broken_code" -ne 0 ]] || fail "broken stub exited 0"
assert_fail_lines "$broken_log"

[[ "$banner_code" -ne 0 ]] || fail "usage banner exited 0"
assert_only_v4_fails "$banner_log" "usage banner"
grep -q 'turn screen lock off' "$banner_log" || fail "usage banner did not quote sysadminctl"

[[ "$doctor_code" -ne 0 ]] || fail "doctor omission exited 0"
assert_only_v4_fails "$doctor_log" "doctor omission"

printf '%s\n' 'test-verify ok'
