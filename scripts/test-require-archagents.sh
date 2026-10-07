#!/usr/bin/env bash
set -euo pipefail

hook=$(cd "$(dirname "$0")/.." && pwd)/.claude/hooks/require-archagents.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/gh"
bash_bin=$(command -v bash)
cat >"$work/bin/gh" <<'STUB'
#!/bin/sh
echo call >>"$STUB_CALLS"
[ "$STUB_EXIT" = 0 ] || exit "$STUB_EXIT"
echo "$STUB_LOGIN"
STUB
chmod +x "$work/bin/gh"

fail=0
pass() { printf 'PASS %s\n' "$1"; }
bad() { printf 'FAIL %s\n%s\n' "$1" "$2"; fail=1; }

run() {
  local mode=$1 event=PreToolUse
  shift
  [[ $mode == session ]] && event=SessionStart
  : >"$work/calls"
  set +e
  printf '{"hook_event_name":"%s"}' "$event" |
    env -i PATH="$work/bin:/usr/bin:/bin" HOME="$work" TMPDIR="$work/tmp" GH_CONFIG_DIR="$work/gh" \
      STUB_CALLS="$work/calls" STUB_LOGIN=ArchAgents STUB_EXIT=0 "$@" \
      "$bash_bin" "$hook" "$mode" >"$work/out" 2>"$work/err"
  code=$?
  set -e
  out=$(cat "$work/out")
  err=$(cat "$work/err")
  calls=$(wc -l <"$work/calls" | tr -d ' ')
}

expect() {
  local name=$1 want_code=$2 want_calls=$3 glob=$4 mode=$5
  shift 5
  run "$mode" "$@"
  local got=$out$err
  if [[ $code -ne $want_code ]]; then
    bad "$name" "code=$code want=$want_code out=$out err=$err"
    return
  fi
  if [[ $want_calls != - && $calls -ne $want_calls ]]; then
    bad "$name" "calls=$calls want=$want_calls out=$out err=$err"
    return
  fi
  if [[ $glob != - && $got != *"$glob"* ]]; then
    bad "$name" "missing [$glob] out=$out err=$err"
    return
  fi
  pass "$name"
}

cold() {
  rm -rf "$work/tmp" "$work/gh"
  mkdir -p "$work/tmp" "$work/gh"
  expect "$@"
}

cold 'ArchAgents is allowed' 0 1 - pretool
expect 'second pretool reads the cache' 0 0 - pretool

cold 'GoddyB is blocked with exit 2' 2 1 'gh is GoddyB, not ArchAgents' pretool STUB_LOGIN=GoddyB
cold 'gh failure is blocked' 2 1 'could not check' pretool STUB_EXIT=1
cold 'missing gh is blocked' 2 1 'could not check' pretool STUB_EXIT=127

cold 'warm cache before a token change' 0 1 - pretool
expect 'a new GH_TOKEN forces a fresh lookup' 2 1 'gh is GoddyB, not ArchAgents' pretool STUB_LOGIN=GoddyB GH_TOKEN=other

cold 'warm cache before hosts.yml changes' 0 1 - pretool
echo 'user: GoddyB' >"$work/gh/hosts.yml"
expect 'gh auth switch (hosts.yml change) forces a fresh lookup' 2 1 'gh is GoddyB, not ArchAgents' pretool STUB_LOGIN=GoddyB

cold 'a failed lookup is not cached' 2 1 'could not check' pretool STUB_EXIT=1
expect 'a failed lookup is not cached' 0 1 - pretool

cold 'session start reports a mismatch' 0 1 'gh is GoddyB, not ArchAgents' session STUB_LOGIN=GoddyB
cold 'session start could not check' 0 1 'could not check' session STUB_EXIT=1
cold 'session start reports a pass' 0 1 'passed' session
expect 'session mode looks up on every start' 0 1 'passed' session

cold 'a Cursor agent outside Claude Code is not gated' 0 0 - pretool STUB_LOGIN=GoddyB CURSOR_VERSION=3.0
cold 'Claude Code inside Cursor is gated' 2 1 'gh is GoddyB, not ArchAgents' pretool STUB_LOGIN=GoddyB CURSOR_VERSION=3.0 CLAUDECODE=1

exit $fail
