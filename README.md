# arch-caffeinate

A macOS command-line tool that keeps a Mac awake while it is on AC power, turns the display fully off after a stretch of inactivity to prevent burn-in, and lets the display wake on mouse movement, a keypress, or a notification without a password prompt.

Status: released as v1, open source under the [LICENSE](LICENSE). The owner's spec lives in `verbatim/`.

This repo is also a workspace for Cursor cloud agents. See `AGENTS.md`.

## Install

From a clone:

```bash
go build ./cmd/arch-caffeinate && ./arch-caffeinate install
```

`install` copies the binary you just ran to `~/.local/bin/arch-caffeinate` and writes `~/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist`. It does not load the agent, and it prints one line that says to run `arch-caffeinate startup install`. Pass `--idle-seconds N` to set the idle threshold for the agent. The default is 600 seconds. A login bash that puts `~/.local/bin` on `PATH` can then run `arch-caffeinate`.

## Start at login

```bash
arch-caffeinate startup install
```

`startup install` loads the plist that `install` wrote, so the agent starts at login. `startup remove` unloads the agent and deletes nothing. Both commands are idempotent, and `startup remove` succeeds when the agent is not loaded. `arch-caffeinate install --startup` runs the install and the load in one step.

## Usage

Commands, status fields, and files are specified in [docs/cli-contract.md](docs/cli-contract.md). How the daemon decides to sleep or wake the display is in [docs/design.md](docs/design.md).

## Uninstall

```bash
arch-caffeinate uninstall
rm -f ~/.local/bin/arch-caffeinate
```

`uninstall` unloads the agent when it is loaded and removes the plist. It works whether or not the agent is loaded. Remove the binary yourself if you do not want it on `PATH`.

## Screen lock

Turning the display fully off locks the Mac unless the macOS screen-lock setting is off. Changing that setting needs the owner's password.

```bash
sysadminctl -screenLock off -password -
```

`status` and `doctor` report the current setting. They do not change it.
