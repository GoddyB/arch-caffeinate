# arch-caffeinate

A macOS command-line tool that keeps a Mac awake while it is on AC power, turns the display fully off after a stretch of inactivity to prevent burn-in, and lets the display wake on mouse movement, a keypress, or a notification without a password prompt.

Status is under construction. The owner's spec lives in `verbatim/`.

This repo is also a workspace for Cursor cloud agents. See `AGENTS.md`.

## Install

From a clone:

```bash
go build ./cmd/arch-caffeinate && ./arch-caffeinate install
```

`install` copies the binary you just ran to `~/.local/bin/arch-caffeinate`, writes `~/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist`, and loads it with `launchctl bootstrap`. Pass `--idle-seconds N` to set the idle threshold for the agent. The default is 600 seconds. A login bash that puts `~/.local/bin` on `PATH` can then run `arch-caffeinate`.

## Usage

`arch-caffeinate install [--idle-seconds N]` copies the running binary, writes the LaunchAgent, and loads it. Running it again replaces the plist and loads the same label.

`arch-caffeinate uninstall` unloads the agent and removes the plist. The binary stays in `~/.local/bin`.

`arch-caffeinate start` loads the LaunchAgent. `arch-caffeinate stop` unloads it.

`arch-caffeinate run [--idle-seconds N] [--poll-ms M]` is the foreground daemon. The default idle threshold is 600 seconds. The default poll is 1000 ms.

`arch-caffeinate status` prints the daemon state. `arch-caffeinate status --json` prints `running`, `pid`, `power`, `sleepPrevented`, `display`, `idleSeconds`, `idleThresholdSeconds`, `screenLock`, and `version`. `display` is the `CurrentPowerState` reading from `ioreg -r -d 1 -c AppleCLCD2`. `1` is `on`. `0` is `off`.

`arch-caffeinate doctor` prints one `PASS`, `WARN`, or `FAIL` line per check and exits 0 unless a line is `FAIL`. On a system that is not macOS the macOS line is `FAIL` and says `not macOS`.

`arch-caffeinate wake` declares user activity so the display turns on. The effect matches `caffeinate -u -t 1`.

`arch-caffeinate --version` prints the version and exits 0.

## Uninstall

```bash
arch-caffeinate uninstall
rm -f ~/.local/bin/arch-caffeinate
```

`uninstall` removes the LaunchAgent. Remove the binary yourself if you do not want it on `PATH`.

## Screen lock

Turning the display fully off locks the Mac unless the macOS screen-lock setting is off. Changing that setting needs the owner's password.

```bash
sysadminctl -screenLock off -password -
```

`status` and `doctor` report the current setting. They do not change it.
