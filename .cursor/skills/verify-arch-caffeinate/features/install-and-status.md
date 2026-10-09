# Install and status

`install` on the owner's Mac writes the binary and the plist without loading the LaunchAgent. `startup install` loads it. `status --json` returns the contract fields. `doctor` exits 0.

## Sub-features

- `v7-install` copies the binary and writes the plist, and a second `install` leaves the same plist. Neither run loads the agent.
- `v7-startup` loads the agent with `startup install` and unloads it with `startup remove`. `status --json` reports each state.
- `v7-status` prints the JSON fields in the contract.
- `v7-doctor` exits 0 with no `FAIL` line.

## How to get to it (user POV)

- Run `arch-caffeinate install` in a terminal on the owner's Mac; it prints one line telling you to run `arch-caffeinate startup install`.
- Run `arch-caffeinate startup install` to load the agent.
- Run `arch-caffeinate status` to see whether sleep is prevented and whether the display is on.

## Driving it with bash

Preconditions:

- The binary you invoke is the one just built for this Mac.
- The login user is the owner.

- **Install.** Run `arch-caffeinate startup remove`, then `arch-caffeinate install --idle-seconds 5` twice. The two plist files are byte-identical. After both, `launchctl print "gui/$UID/io.github.goddyb.arch-caffeinate"` exits non-zero: `install` does not load the agent.
- **Startup install.** Run `arch-caffeinate startup install`. `launchctl print "gui/$UID/io.github.goddyb.arch-caffeinate"` exits 0 and its `pid = N` line equals `pid` in `status --json`. `running` is true and `idleThresholdSeconds` is `5`.
- **Startup remove.** Run `arch-caffeinate startup remove`. Within 8 seconds `running` is false and `launchctl print` exits non-zero. The plist and the binary are still present.
- **Status.** `running` is true while the agent is loaded. `power` is `ac`, `battery`, or `unknown`. `display` is `on` or `off`. `version` equals `arch-caffeinate --version`.
- **Doctor.** The exit code is 0, and every line starts with `PASS` or `WARN`. Record the exit code from the command itself. Do not append `|| true` before reading `$?`.
- **Version.** Run `arch-caffeinate --version`. The exit code is 0 and the text matches `version` in the JSON.
- **Proof.** Save the plist path contents, the `launchctl print` excerpts for loaded and unloaded, `status.json` for each state, and the doctor transcript.

## Gotchas

- `install` does not build and does not load. A missing binary is `FAIL`, not a cue to compile inside the proof.
- `startup install` without a plist fails. Run `install` first.
- Cleanup runs `arch-caffeinate install` without `--idle-seconds`, then restores the load state (`startup install` if loaded, `startup remove` if not). It waits until `idleThresholdSeconds` is `600` and the plist has no `--idle-seconds` argument. Once the old heartbeat is dead, `status` reports the plist threshold; a live one can still report the old threshold.
- Leave the evidence directory in place after cleanup.
