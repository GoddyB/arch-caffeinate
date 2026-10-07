#!/usr/bin/env bash
# Exercises scripts/check-public-hygiene.sh in throwaway repositories.
set -euo pipefail

src=$(cd "$(dirname "$0")" && pwd)/check-public-hygiene.sh
fail=0

nore='324734221+GoddyB@users.noreply.github.com'
gmail='bob@gmail.com'

pass() { printf 'PASS %s\n' "$1"; }
bad() { printf 'FAIL %s\n%s\n' "$1" "$2"; fail=1; }

new_repo() {
  local dir
  dir=$(mktemp -d)
  mkdir -p "$dir/scripts"
  cp "$src" "$dir/scripts/check-public-hygiene.sh"
  chmod +x "$dir/scripts/check-public-hygiene.sh"
  git -C "$dir" init -q
  git -C "$dir" config user.name GoddyB
  git -C "$dir" config user.email "$nore"
  printf '%s\n' "$dir"
}

commit_all() {
  local dir=$1 msg=$2
  git -C "$dir" add -A
  git -C "$dir" commit -qm "$msg"
}

run_check() {
  local dir=$1
  set +e
  out=$(cd "$dir" && bash scripts/check-public-hygiene.sh 2>&1)
  code=$?
  set -e
}

expect() {
  local name=$1 want=$2 dir=$3
  run_check "$dir"
  if [[ $code -ne $want ]]; then
    bad "$name" "exit $code want $want"$'\n'"$out"
    return
  fi
  pass "$name"
}

dir=$(new_repo)
printf '%s\n' 'hello' > "$dir/README.md"
commit_all "$dir" 'clean'
expect 'clean repo passes' 0 "$dir"

dir=$(new_repo)
printf '%s\n' 'hello' > "$dir/README.md"
commit_all "$dir" 'base'
git -C "$dir" config user.email "$gmail"
printf '%s\n' 'more' >> "$dir/README.md"
commit_all "$dir" 'gmail identity'
run_check "$dir"
if [[ $code -eq 0 ]] || ! printf '%s\n' "$out" | grep -q 'non-noreply' || printf '%s\n' "$out" | grep -q "$gmail"; then
  bad 'gmail commit identity fails' "exit $code"$'\n'"$out"
else
  pass 'gmail commit identity fails'
fi

dir=$(new_repo)
printf '%s\n' 'hello' > "$dir/README.md"
git -C "$dir" add -A
git -C "$dir" commit -q -m 'trailer' -m "Co-authored-by: Bob <$gmail>"
expect 'email in a commit message fails' 1 "$dir"

dir=$(new_repo)
printf '%s\n' 'hello' > "$dir/README.md"
commit_all "$dir" 'base'
git -C "$dir" config user.email "$gmail"
printf '%s\n' 'leak' >> "$dir/README.md"
commit_all "$dir" 'known leak commit'
sha=$(git -C "$dir" rev-parse HEAD)
git -C "$dir" config user.email "$nore"
printf '%s\n' "$sha" > "$dir/.hygiene-known-leaks"
commit_all "$dir" 'record known leak'
run_check "$dir"
if [[ $code -ne 0 ]] || ! printf '%s\n' "$out" | grep -q 'WARNING'; then
  bad 'known leak warns and passes' "exit $code"$'\n'"$out"
else
  pass 'known leak warns and passes'
fi

dir=$(new_repo)
printf '%s\n' "keep ${nore} and ${gmail}" > "$dir/notes.txt"
commit_all "$dir" 'mixed line'
run_check "$dir"
if [[ $code -eq 0 ]] || ! printf '%s\n' "$out" | grep -q "$gmail"; then
  bad 'mixed noreply and gmail line fails' "exit $code"$'\n'"$out"
else
  pass 'mixed noreply and gmail line fails'
fi

dir=$(new_repo)
printf '%s\n' 'hello' > "$dir/README.md"
commit_all "$dir" 'base'
printf '%s\n' 'ref: refs/heads/no-such' > "$dir/.git/HEAD"
run_check "$dir"
if [[ $code -eq 0 ]] || ! printf '%s\n' "$out" | grep -q 'git log failed'; then
  bad 'git log failure exits non-zero' "exit $code"$'\n'"$out"
else
  pass 'git log failure exits non-zero'
fi

if [[ $fail -ne 0 ]]; then
  exit 1
fi
