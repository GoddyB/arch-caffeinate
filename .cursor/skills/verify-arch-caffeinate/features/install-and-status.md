# Install and status

V7. "Once built, install on my computer; I am first user; test it."

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

- **Install.** Run `arch-caffeinate install --idle-seconds 5` twice. Then run `launchctl print "gui/$UID/io.github.goddyb.arch-caffeinate"`. The agent is loaded. The plist program arguments include `run` and `--idle-seconds` `5`.
- **Status.** Run `arch-caffeinate status --json`. The object contains `running`, `pid`, `power`, `sleepPrevented`, `display`, `idleSeconds`, `idleThresholdSeconds`, `screenLock`, and `version`. `idleThresholdSeconds` is `5` during this proof.
- **Doctor.** Run `arch-caffeinate doctor`. The exit code is 0.
- **Version.** Run `arch-caffeinate --version`. The exit code is 0 and the text matches `version` in the JSON.
- **Proof.** Save the plist path contents, the `launchctl print` excerpt, `status.json`, and the doctor transcript.

## Gotchas

- `install` does not build. A missing binary is `FAIL`, not a cue to compile inside the proof.
- Cleanup runs `arch-caffeinate install` without `--idle-seconds` and checks that `idleThresholdSeconds` is `600`.
- Leave the evidence directory in place after cleanup.
