#!/usr/bin/env bash
# Claude Code blocks a tool only on exit 2. Any other non-zero exit is a
# non-blocking error, and the tool runs. SessionStart can't block.
set -uo pipefail
trap '(( $? == 0 )) || exit 2' EXIT

if [[ -z ${CLAUDECODE:-} && -n ${CURSOR_VERSION:-} ]]; then
  exit 0
fi

want=ArchAgents
mode=${1:-pretool}
cat >/dev/null

hosts="${GH_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gh}/hosts.yml"
key=$({ printf '%s\n%s\n' "${GH_TOKEN:-}" "${GITHUB_TOKEN:-}"; cat "$hosts" 2>/dev/null; } | cksum | cut -d ' ' -f 1)
dir="${TMPDIR:-/tmp}/claude-archagents-$(id -u)"
cache="$dir/$key"
mkdir -p "$dir" && chmod 700 "$dir" || exit 2

# lookup_deadline_s stays under the 30s hook timeout in .claude/settings.json.
# A hook that hits that timeout does not block the tool.
lookup_deadline_s=6

lookup() {
  local out="$dir/$$.out" pid i
  gh api user --jq .login >"$out" 2>/dev/null &
  pid=$!
  for ((i = 0; i < lookup_deadline_s * 5; i++)); do
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
if [[ $mode == pretool && -s $cache ]]; then
  login=$(<"$cache")
elif login=$(lookup) && [[ -n $login ]]; then
  printf '%s' "$login" >"$cache.$$" && mv -f "$cache.$$" "$cache"
fi

if [[ $login == "$want" ]]; then
  [[ $mode == session ]] && echo "GitHub identity check passed: gh is $want."
  exit 0
fi

if [[ -z $login ]]; then
  msg="GitHub identity check could not check who gh is. gh is missing, logged out, or unreachable. Claude Code works in this repo only as $want. Stop and report this; don't switch accounts or borrow another token."
else
  msg="GitHub identity check failed: gh is $login, not $want. Claude Code works in this repo only as $want. Stop and report this; don't switch accounts or borrow another token."
fi
if [[ $mode == session ]]; then
  echo "$msg Every tool call in this session will be blocked."
  exit 0
fi
echo "$msg" >&2
exit 2
