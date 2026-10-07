#!/usr/bin/env bash
# Fails when tracked files contain personal data that must not ship in a public repo.
set -euo pipefail

if ! top=$(git rev-parse --show-toplevel); then
  echo "public-hygiene: git rev-parse failed" >&2
  exit 1
fi
cd "$top"

patterns=(
  '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
  '/Users/[A-Za-z0-9._-]+'
  '/home/[A-Za-z0-9._-]+/'
  '\b100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[0-9]{1,3}\.[0-9]{1,3}\b'
  '[A-Za-z0-9-]+\.ts\.net'
  '(gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,}|key_[A-Za-z0-9]{24,})'
)
commit_email_allow='users\.noreply\.github\.com$|^noreply@github\.com$|^cursoragent@cursor\.com$'
# shellcheck disable=SC2016 # literal $USER patterns, not expansions
allow='(users\.noreply\.github\.com|noreply@github\.com|cursoragent@cursor\.com|/Users/(example|you|USER|\$USER|\$\{USER\})|/home/(runner|example|user|ubuntu)/|@example\.(com|org)|git@github\.com)'
status=0
for p in "${patterns[@]}"; do
  set +e
  raw=$(git grep -noIE "$p" -- . ':!scripts/check-public-hygiene.sh' ':!scripts/test-public-hygiene.sh')
  gcode=$?
  set -e
  if [[ $gcode -gt 1 ]]; then
    echo "public-hygiene: git grep failed" >&2
    exit 1
  fi
  hits=''
  if [[ $gcode -eq 0 ]]; then
    while IFS= read -r line; do
      [[ -z $line ]] && continue
      match=${line#*:}
      match=${match#*:}
      if printf '%s\n' "$match" | grep -qE "$allow"; then
        continue
      fi
      hits+="$line"$'\n'
    done <<< "$raw"
  fi
  if [[ -n $hits ]]; then
    echo "public-hygiene: forbidden pattern found:"
    printf '%s' "$hits"
    status=1
  fi
done

# Every commit in history is checked: author, committer, and any email in the message
# (Cursor appends a Co-authored-by trailer with the owner's profile email).
if ! log_out=$(git log --format='%H%x09%ae%x09%ce'); then
  echo "public-hygiene: git log failed" >&2
  exit 1
fi
if ! shas=$(git rev-list HEAD); then
  echo "public-hygiene: git rev-list failed" >&2
  exit 1
fi

known=''
if [[ -f .hygiene-known-leaks ]]; then
  known=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' .hygiene-known-leaks)
fi

if [[ -n $log_out ]]; then
  while IFS=$'\t' read -r sha ae ce; do
    [[ -z ${sha:-} ]] && continue
    for email in "$ae" "$ce"; do
      [[ -z $email ]] && continue
      if printf '%s\n' "$email" | grep -qE "$commit_email_allow"; then
        continue
      fi
      if printf '%s\n' "$known" | grep -qxF "$sha"; then
        echo "public-hygiene: WARNING known leak awaiting history rewrite: $sha"
      else
        echo "public-hygiene: commit $sha has a non-noreply identity"
        status=1
      fi
    done
  done <<< "$log_out"
fi
# Report leaks by commit only: CI logs are public, so never echo the address itself.
for sha in $shas; do
  for email in $(git log -1 --format=%B "$sha" | grep -oE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' || true); do
    if printf '%s\n' "$email" | grep -qE "$commit_email_allow"; then
      continue
    fi
    if printf '%s\n' "$known" | grep -qxF "$sha"; then
      echo "public-hygiene: WARNING known leak awaiting history rewrite: $sha (message)"
    else
      echo "public-hygiene: commit $sha has an email in its message"
      status=1
    fi
  done
done
exit $status
