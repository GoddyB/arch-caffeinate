#!/usr/bin/env bash
# Claude Code hook: allow work in this repo only as the GitHub user ArchAgents.
# Usage: require-archagents.sh pretool|session   (hook input JSON on stdin)
#   pretool  exits 2, which blocks the tool call, unless `gh api user` says ArchAgents.
#   session  prints the check result into the session context. SessionStart can't block.
# The login is cached per session, per GH_TOKEN/GITHUB_TOKEN, and per gh hosts.yml,
# so `gh auth switch` or a new token forces a fresh lookup.
set -uo pipefail

want=ArchAgents
mode=${1:-pretool}
input=$(cat)

sid=$(printf '%s' "$input" | sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([A-Za-z0-9_-]*\)".*/\1/p' | head -n 1)
[[ -n $sid ]] || sid=nosession
hosts="${GH_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gh}/hosts.yml"
key=$({ printf '%s\n%s\n' "${GH_TOKEN:-}" "${GITHUB_TOKEN:-}"; cat "$hosts" 2>/dev/null; } | cksum | cut -d ' ' -f 1)
dir="${TMPDIR:-/tmp}/claude-archagents-$(id -u)"
cache="$dir/$sid-$key"

lookup() {
  local out="$dir/$sid-$$.out" pid i
  command -v gh >/dev/null 2>&1 || return 1
  mkdir -p "$dir" && chmod 700 "$dir" || return 1
  gh api user --jq .login >"$out" 2>/dev/null &
  pid=$!
  for ((i = 0; i < 50; i++)); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.2
  done
  kill "$pid" 2>/dev/null
  if ! wait "$pid"; then
    rm -f "$out"
    return 1
  fi
  tr -d '[:space:]' <"$out"
  rm -f "$out"
}

login=''
if [[ -f $cache ]]; then
  login=$(cat "$cache")
else
  if login=$(lookup) && [[ -n $login ]]; then
    printf '%s' "$login" >"$cache"
  else
    login=''
  fi
fi

if [[ $login == "$want" ]]; then
  [[ $mode == session ]] && echo "GitHub identity check passed: gh is ArchAgents."
  exit 0
fi

seen=${login:-unknown (gh missing, logged out, or unreachable)}
msg="GitHub identity check failed: gh is $seen, not $want. Claude Code works in this repo only as $want. Stop and report this; don't switch accounts or borrow another token."
if [[ $mode == session ]]; then
  echo "$msg Every tool call in this session will be blocked."
  exit 0
fi
echo "$msg" >&2
exit 2
