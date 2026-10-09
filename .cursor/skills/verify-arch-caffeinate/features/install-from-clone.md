# Install from clone

A clean clone builds `arch-caffeinate` and proves the opt-in install flow against a stub `launchctl`. `install` writes the binary and plist without bootstrapping; `startup install` records the bootstrap and `startup remove` the bootout.

## Sub-features

- `v8-clone` runs `scripts/test-install-from-clone.sh` and expects exit 0.

## How to get to it (user POV)

- Clone the repo and install from that build, not from an earlier binary.

## Driving it with bash

Preconditions:

- `git` can read `origin`, and the current revision is already on that remote. An unpushed `HEAD` fails.
- Go can build `./cmd/arch-caffeinate`.
- The script uses a temporary `HOME`. It does not touch the login user's LaunchAgents.

- **Run the script.** From the clone, run `scripts/test-install-from-clone.sh`.
- **Pass V8.** The exit code is 0.
- **Proof.** Save the script output under the evidence directory.

## Gotchas

- The script clones `origin`, so it needs network access to that remote.
- Set `VERIFY_INSTALL_TEST` only when a stub should stand in for this script. The Mac proof uses the real path.
