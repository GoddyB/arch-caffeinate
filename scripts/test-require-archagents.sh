#!/usr/bin/env bash
# Exercises .claude/hooks/require-archagents.sh against a stubbed gh.
set -euo pipefail

hook=$(cd "$(dirname "$0")/.." && pwd)/.claude/hooks/require-archagents.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/gh" "$work/nogh"
for tool in cat chmod cksum cut head id mkdir rm sed sleep tr; do
  ln -s "$(command -v "$tool")" "$work/nogh/$tool"
done
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

# run <mode> <session> [VAR=value...]: sets code, out, err, calls
run() {
  local mode=$1 sid=$2
  shift 2
  : >"$work/calls"
  set +e
  printf '{"session_id":"%s","hook_event_name":"PreToolUse"}' "$sid" |
    env -i PATH="$work/bin:/usr/bin:/bin" HOME="$work" TMPDIR="$work/tmp" GH_CONFIG_DIR="$work/gh" \
      STUB_CALLS="$work/calls" STUB_LOGIN=ArchAgents STUB_EXIT=0 "$@" \
      "$bash_bin" "$hook" "$mode" >"$work/out" 2>"$work/err"
  code=$?
  set -e
  out=$(cat "$work/out")
  err=$(cat "$work/err")
  calls=$(wc -l <"$work/calls" | tr -d ' ')
}

run pretool s1
if [[ $code -eq 0 && -z $err ]]; then pass 'ArchAgents is allowed'; else bad 'ArchAgents is allowed' "code=$code err=$err"; fi

run pretool s2 STUB_LOGIN=GoddyB
if [[ $code -eq 2 && $err == *'gh is GoddyB, not ArchAgents'* ]]; then pass 'GoddyB is blocked with exit 2'; else bad 'GoddyB is blocked with exit 2' "code=$code err=$err"; fi

run pretool s3 STUB_EXIT=1
if [[ $code -eq 2 && $err == *unknown* ]]; then pass 'gh failure is blocked'; else bad 'gh failure is blocked' "code=$code err=$err"; fi

run pretool s4 PATH="$work/nogh"
if [[ $code -eq 2 && $calls -eq 0 && $err == *unknown* ]]; then pass 'missing gh is blocked'; else bad 'missing gh is blocked' "code=$code calls=$calls err=$err"; fi

run pretool s5
run pretool s5
if [[ $code -eq 0 && $calls -eq 0 ]]; then pass 'second call in a session uses the cache'; else bad 'second call in a session uses the cache' "code=$code calls=$calls"; fi

run pretool s5 STUB_LOGIN=GoddyB GH_TOKEN=other
if [[ $code -eq 2 && $calls -eq 1 ]]; then pass 'a new GH_TOKEN forces a fresh lookup'; else bad 'a new GH_TOKEN forces a fresh lookup' "code=$code calls=$calls"; fi

echo 'user: GoddyB' >"$work/gh/hosts.yml"
run pretool s5 STUB_LOGIN=GoddyB
if [[ $code -eq 2 && $calls -eq 1 ]]; then pass 'gh auth switch (hosts.yml change) forces a fresh lookup'; else bad 'gh auth switch (hosts.yml change) forces a fresh lookup' "code=$code calls=$calls"; fi

run pretool s6 STUB_EXIT=1
run pretool s6
if [[ $code -eq 0 && $calls -eq 1 ]]; then pass 'a failed lookup is not cached'; else bad 'a failed lookup is not cached' "code=$code calls=$calls"; fi

run session s7 STUB_LOGIN=GoddyB
if [[ $code -eq 0 && $out == *'will be blocked'* ]]; then pass 'session start reports a mismatch'; else bad 'session start reports a mismatch' "code=$code out=$out"; fi

run session s8
if [[ $code -eq 0 && $out == *passed* ]]; then pass 'session start reports a pass'; else bad 'session start reports a pass' "code=$code out=$out"; fi

run pretool c1 STUB_LOGIN=GoddyB CURSOR_VERSION=3.0
if [[ $code -eq 0 && $calls -eq 0 ]]; then pass 'a Cursor agent outside Claude Code is not gated'; else bad 'a Cursor agent outside Claude Code is not gated' "code=$code calls=$calls"; fi

run pretool c2 STUB_LOGIN=GoddyB CURSOR_VERSION=3.0 CLAUDECODE=1
if [[ $code -eq 2 ]]; then pass 'Claude Code inside Cursor is gated'; else bad 'Claude Code inside Cursor is gated' "code=$code err=$err"; fi

exit $fail
