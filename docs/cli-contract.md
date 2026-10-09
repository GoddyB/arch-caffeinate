# arch-caffeinate CLI contract

`arch-caffeinate` is a macOS command-line tool written in Go. The build implements this contract. The verification skill drives the installed binary against it.

Paths below use the login user's home directory. The binary name on `PATH` is `arch-caffeinate`.

## Commands

`arch-caffeinate install [--idle-seconds N] [--startup]` builds nothing. It copies the running binary to `~/.local/bin/arch-caffeinate` and writes `~/Library/LaunchAgents/io.github.goddyb.arch-caffeinate.plist`. It does not load the agent. When it finishes without loading, it prints one line that says to run `arch-caffeinate startup install`. The plist sets `RunAtLoad` and `KeepAlive`. Its program arguments are `arch-caffeinate run`, plus `--idle-seconds N` when that flag is present. Running `install` again replaces the plist and leaves the load state alone. The end state matches the flags of the latest `install`. `--startup` also loads the agent, the same as `startup install` after `install`.

`arch-caffeinate startup install` loads the already-written plist with `launchctl bootstrap gui/$UID`. The load is idempotent. An already-loaded agent is booted out and bootstrapped again. When the plist is missing, the command fails and says to run `install` first.

`arch-caffeinate startup remove` boots the agent out with `launchctl bootout`. It removes neither the plist nor the binary. It is idempotent and succeeds when the agent is not loaded.

`arch-caffeinate uninstall` boots the agent out and removes the plist, whether or not the agent is loaded. It leaves `~/.local/bin/arch-caffeinate` in place.

`arch-caffeinate run [--idle-seconds N] [--poll-ms M]` is the foreground daemon the LaunchAgent runs. The default idle threshold is 600 seconds. The default poll is 1000 ms.

`arch-caffeinate status [--json]` prints daemon state. With `--json` the object has these fields.

| Field | Value |
| --- | --- |
| `running` | bool |
| `pid` | int or null |
| `power` | `"ac"`, `"battery"`, or `"unknown"` |
| `sleepPrevented` | bool, true only while the daemon holds a system-sleep assertion |
| `display` | `"on"` or `"off"` |
| `idleSeconds` | number |
| `idleThresholdSeconds` | int |
| `screenLock` | `"off"`, `"immediate"`, `"delay:<seconds>"`, or `"unknown"` |
| `version` | string |

`arch-caffeinate doctor` runs read-only checks and prints one line per check. Each line contains `PASS`, `WARN`, or `FAIL`. The checks are macOS, the binary on `PATH` in a login bash, the LaunchAgent loaded, a fresh daemon heartbeat, a readable power source, a readable idle time, and the screen-lock setting. The process exits 0 unless a line is `FAIL`. On a system that is not macOS the macOS check is `FAIL` and the line says `not macOS`.

`arch-caffeinate wake` declares user activity so the display turns on. The effect matches `caffeinate -u -t 1`.

`arch-caffeinate --version` prints the version string and exits 0.

## Files the daemon writes

Each poll, the daemon writes `~/Library/Application Support/arch-caffeinate/state.json`. The file holds `pid`, `sleepPrevented`, `idleThresholdSeconds`, `writtenAt` (unix time in milliseconds), and `pollMs` (the poll interval in milliseconds). `status` reads power, idle, display, and screen lock live. The heartbeat is fresh when `pollMs` is positive, `writtenAt` is within three poll intervals of now, and the pid in the file is alive.

The daemon appends logs to `~/Library/Logs/arch-caffeinate.log`.

## Behavior

On AC power the daemon holds a system-sleep assertion, the same kind `caffeinate -s` holds. On battery it releases that assertion.

When HID idle time reaches the idle threshold, the daemon turns the display fully off once, the same effect as `pmset displaysleepnow`. It does not leave the display in the macOS dim state. Mouse movement or a keypress wakes the display through macOS. A delivered notification makes the daemon declare user activity so the display turns on.

The display wakes without a password prompt only when the macOS screen-lock setting is off. `status` and `doctor` report that setting.
