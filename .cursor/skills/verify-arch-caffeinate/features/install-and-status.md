# Install and status

`install` on the owner's Mac loads the LaunchAgent. `status --json` returns the contract fields. `doctor` exits 0.

## Sub-features

- `v7-install` copies the binary, writes the plist, and bootstraps the agent. A second `install` leaves that same end state.
- `v7-status` prints the JSON fields in the contract.
- `v7-doctor` exits 0 with no `FAIL` line.

## How to get to it (user POV)

- Run `arch-caffeinate install` in a terminal on the owner's Mac.
- Run `arch-caffeinate status` to see whether sleep is prevented and whether the display is on.

## Driving it with bash

Preconditions:

- The binary you invoke is the one just built for this Mac.
- The login user is the owner.

- **Install.** Run `arch-caffeinate install --idle-seconds 5` twice. The two plist files are byte-identical. `launchctl print "gui/$UID/io.github.goddyb.arch-caffeinate"` exits 0 and its `pid = N` line equals `pid` in `status --json`.
- **Status.** `running` is true. `idleThresholdSeconds` is `5`. `power` is `ac`, `battery`, or `unknown`. `display` is `on` or `off`. `version` equals `arch-caffeinate --version`.
- **Doctor.** The exit code is 0, and every line starts with `PASS` or `WARN`. Record the exit code from the command itself. Do not append `|| true` before reading `$?`.
- **Version.** Run `arch-caffeinate --version`. The exit code is 0 and the text matches `version` in the JSON.
- **Proof.** Save the plist path contents, the `launchctl print` excerpt, `status.json`, and the doctor transcript.

## Gotchas

- `install` does not build. A missing binary is `FAIL`, not a cue to compile inside the proof.
- Cleanup runs `arch-caffeinate install` without `--idle-seconds`, waits until `idleThresholdSeconds` is `600`, and checks that the plist has no `--idle-seconds` argument. Once the previous heartbeat is dead, `status` reports the threshold from the plist. A heartbeat that is still live can still report the previous threshold.
- Leave the evidence directory in place after cleanup.
