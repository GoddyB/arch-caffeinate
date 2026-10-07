#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
ORIGIN="${ARCH_CAFFEINATE_CLONE_URL:-$(git -C "$ROOT" remote get-url origin)}"
REV="${ARCH_CAFFEINATE_CLONE_REV:-$(git -C "$ROOT" rev-parse HEAD)}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

git clone --quiet "$ORIGIN" "$WORKDIR/src"
if ! git -C "$WORKDIR/src" cat-file -e "$REV^{commit}" 2>/dev/null; then
  git -C "$WORKDIR/src" fetch --quiet origin "$REV"
fi
git -C "$WORKDIR/src" checkout --quiet "$REV"

(
  cd "$WORKDIR/src"
  go build -o "$WORKDIR/arch-caffeinate" ./cmd/arch-caffeinate
)

HOME_DIR="$(mktemp -d)"
mkdir -p "$HOME_DIR"
cat >"$HOME_DIR/.bash_profile" <<'EOF'
export PATH="$HOME/.local/bin:$PATH"
EOF

LOG="$WORKDIR/launchctl.log"
STUB="$WORKDIR/launchctl"
cat >"$STUB" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$LOG"
exit 0
EOF
chmod +x "$STUB"

HOME="$HOME_DIR" ARCH_CAFFEINATE_LAUNCHCTL="$STUB" "$WORKDIR/arch-caffeinate" install --idle-seconds 5

test -x "$HOME_DIR/.local/bin/arch-caffeinate"
test -f "$HOME_DIR/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist"
grep -q 'bootstrap gui/' "$LOG"

RESOLVED="$(HOME="$HOME_DIR" bash -lc 'command -v arch-caffeinate')"
test "$RESOLVED" = "$HOME_DIR/.local/bin/arch-caffeinate"
echo "install-from-clone ok $RESOLVED"
