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
# HYGIENE_RANGE limits the commit check to new commits (CI sets it). Unset checks all history.
emails=$(git log --format='%ae%n%ce' ${HYGIENE_RANGE:-} 2>/dev/null | sort -u || true)
if bad=$(printf '%s\n' "$emails" | grep -vE 'users\.noreply\.github\.com$|^noreply@github\.com$|^cursoragent@cursor\.com$|^$'); then
  echo "public-hygiene: non-noreply commit email(s):"; echo "$bad"; status=1
fi
exit $status
