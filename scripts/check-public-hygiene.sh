#!/usr/bin/env bash
# Fails when tracked files contain personal data that must not ship in a public repo.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
patterns=(
  '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
  '/Users/[A-Za-z0-9._-]+'
  '/home/[A-Za-z0-9._-]+/'
  '\b100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[0-9]{1,3}\.[0-9]{1,3}\b'
  '[A-Za-z0-9-]+\.ts\.net'
  '(gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,}|key_[A-Za-z0-9]{24,})'
)
allow='(users\.noreply\.github\.com|/Users/(example|you|USER|\$USER|\$\{USER\})|/home/(runner|example|user|ubuntu)/|@example\.(com|org)|git@github\.com)'
status=0
for p in "${patterns[@]}"; do
  if hits=$(git grep -nIE "$p" -- . ':!scripts/check-public-hygiene.sh' | grep -vE "$allow"); then
    echo "public-hygiene: forbidden pattern found:"; echo "$hits"; status=1
  fi
done
# Every commit in history must carry a noreply identity. Commits listed in
# .hygiene-known-leaks predate the guard and await an owner-approved history
# rewrite; they are reported on every run but do not fail it.
known=$(grep -oE '^[0-9a-f]{40}' .hygiene-known-leaks 2>/dev/null || true)
while IFS='|' read -r sha ae ce; do
  [ -z "$sha" ] && continue
  for e in "$ae" "$ce"; do
    if ! printf '%s\n' "$e" | grep -qE 'users\.noreply\.github\.com$|^noreply@github\.com$|^cursoragent@cursor\.com$'; then
      if printf '%s\n' "$known" | grep -qx "$sha"; then
        echo "public-hygiene: WARNING known leak awaiting history rewrite: $sha"
      else
        echo "public-hygiene: commit $sha has a non-noreply identity"; status=1
      fi
    fi
  done
done < <(git log --format='%H|%ae|%ce' 2>/dev/null || true)
exit $status
