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
cat "$VERIFY_FIXTURE/assertions.txt"
EOF

  cat >"$bin/ioreg" <<'EOF'
#!/bin/sh
case "$*" in
  *AppleCLCD2*)
    printf '"CurrentPowerState"=%s\n' "$(cat "$VERIFY_FIXTURE/display")"
    ;;
  *IOHIDSystem*)
    printf '%s\n' 'HIDIdleTime = 6000000000'
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
printf 'pid = %s\n' "$(cat "$VERIFY_FIXTURE/parent_pid")"
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
display = open(os.path.join(fix, "display")).read().strip()
cleanup = os.path.exists(os.path.join(fix, "cleanup"))
if mode == "healthy":
    payload = {
        "running": True,
        "pid": int(open(os.path.join(fix, "parent_pid")).read()),
        "power": "ac",
        "sleepPrevented": True,
        "display": "on" if display == "1" else "off",
        "idleSeconds": 6,
        "idleThresholdSeconds": 600 if cleanup else 5,
        "screenLock": "off",
        "version": "0.1.0",
        "writtenAt": int(time.time() * 1000),
    }
else:
    payload = {
        "running": False,
        "pid": None,
        "power": "ac",
        "sleepPrevented": False,
        "display": "on" if display == "1" else "off",
        "idleSeconds": 0,
        "idleThresholdSeconds": 5,
        "screenLock": "immediate",
        "version": "0.1.0",
        "writtenAt": 0,
    }
with open(dest, "w") as f:
    json.dump(payload, f)
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
    printf '%s\n' 'PASS macOS' 'PASS heartbeat' 'WARN screen lock readable'
    ;;
  status)
    write_state
    cat "$state_dir/state.json"
    printf '\n'
    ;;
  wake)
    if [[ "$VERIFY_MODE" == "healthy" ]]; then
      printf '%s\n' 1 >"$fix/display"
    fi
    ;;
  stop)
    if [[ "$VERIFY_MODE" == "healthy" ]]; then
      printf '%s\n' 0 >"$fix/display"
      printf '%s\n' stopped >"$fix/agent_mode"
    fi
    ;;
  start)
    if [[ "$VERIFY_MODE" == "healthy" ]]; then
      printf '%s\n' started >"$fix/agent_mode"
    fi
    ;;
  --version)
    printf '%s\n' '0.1.0'
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
    printf '%s\n' 0 >"$fix/display"
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
    "$work/parent.sh" &
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
    printf '   PreventSystemSleep  1\n   pid %s(caffeinate): PreventSystemSleep named: "arch-caffeinate"\n' "$child" >"$fix/assertions.txt"
  else
    mkdir -p "$home/other/bin"
    cp "$bin/arch-caffeinate" "$home/other/bin/arch-caffeinate"
    printf '%s\n' "export PATH=\"\$HOME/other/bin:\$PATH\"" >"$home/.bash_profile"
    printf '%s\n' 1 >"$fix/display"
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

set +e
healthy_log="$(mktemp)"
broken_log="$(mktemp)"

HOME_RUN="$(mktemp -d)"
prepare healthy "$HOME_RUN" 0
HOME="$HOME_RUN/home" \
  VERIFY_MODE=healthy \
  VERIFY_FIXTURE="$HOME_RUN/fixture" \
  VERIFY_POLL_SEC=0 \
  VERIFY_OUT="$HOME_RUN/out" \
  VERIFY_INSTALL_TEST="$HOME_RUN/install-test.sh" \
  PATH="$HOME_RUN/bin:$PATH" \
  "$VERIFY" >"$healthy_log" 2>&1
healthy_code=$?

BROKEN_RUN="$(mktemp -d)"
prepare broken "$BROKEN_RUN" 1
HOME="$BROKEN_RUN/home" \
  VERIFY_MODE=broken \
  VERIFY_FIXTURE="$BROKEN_RUN/fixture" \
  VERIFY_POLL_SEC=0 \
  VERIFY_OUT="$BROKEN_RUN/out" \
  VERIFY_INSTALL_TEST="$BROKEN_RUN/install-test.sh" \
  PATH="$BROKEN_RUN/bin:$PATH" \
  "$VERIFY" >"$broken_log" 2>&1
broken_code=$?
set -e

if [[ -f "$HOME_RUN/fixture/parent_pid" ]]; then
  kill "$(cat "$HOME_RUN/fixture/parent_pid")" 2>/dev/null || true
  pkill -P "$(cat "$HOME_RUN/fixture/parent_pid")" 2>/dev/null || true
fi

printf '%s\n' "--- healthy exit ${healthy_code} ---"
cat "$healthy_log"
printf '%s\n' "--- broken exit ${broken_code} ---"
cat "$broken_log"

[[ "$healthy_code" -eq 0 ]] || fail "healthy stub exited ${healthy_code}"
grep -q '^FAIL ' "$healthy_log" && fail "healthy stub printed FAIL"
for id in V1 V2 V3 V4 V5 V6 V7 V8; do
  grep -q "^PASS ${id} " "$healthy_log" || fail "missing PASS ${id}"
done

[[ "$broken_code" -ne 0 ]] || fail "broken stub exited 0"
assert_fail_lines "$broken_log"

printf '%s\n' 'test-verify ok'
