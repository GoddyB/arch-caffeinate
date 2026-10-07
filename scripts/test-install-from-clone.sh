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

HOME_DIR="$WORKDIR/home"
mkdir -p "$HOME_DIR"

LOG="$WORKDIR/launchctl.log"
STUB="$WORKDIR/launchctl"
cat >"$STUB" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$LOG"
exit 0
EOF
chmod +x "$STUB"

PLIST="$HOME_DIR/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist"
HOME="$HOME_DIR" ARCH_CAFFEINATE_LAUNCHCTL="$STUB" "$WORKDIR/arch-caffeinate" install --idle-seconds 5
cp "$PLIST" "$WORKDIR/plist.1"
HOME="$HOME_DIR" ARCH_CAFFEINATE_LAUNCHCTL="$STUB" "$WORKDIR/arch-caffeinate" install --idle-seconds 5
cmp "$PLIST" "$WORKDIR/plist.1"
cmp "$WORKDIR/arch-caffeinate" "$HOME_DIR/.local/bin/arch-caffeinate"
grep -qx "bootstrap gui/$(id -u) $PLIST" "$LOG"
grep -q '<string>--idle-seconds</string>' "$PLIST"
grep -q '<key>KeepAlive</key><true></true>' "$PLIST"

echo "install-from-clone ok"
