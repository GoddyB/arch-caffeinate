# Install from clone

V8. "this should probably be included in your verification skill"

A clean clone builds `arch-caffeinate` and `install` loads it against a stub `launchctl`.

## Sub-features

- `v8-clone` runs `scripts/test-install-from-clone.sh` and expects exit 0.

## How to get to it (user POV)

- Clone the repo and install from that build, without reusing a binary left over from an earlier checkout.

## Driving it with bash

Preconditions:

- `git` can read `origin` and the current revision.
- Go can build `./cmd/arch-caffeinate`.
- The script uses a temporary `HOME`. It does not touch the login user's LaunchAgents.

- **Run the script.** From the clone, run `scripts/test-install-from-clone.sh`.
- **Pass V8.** The exit code is 0.
- **Proof.** Save the script output under the evidence directory.

## Gotchas

- The script clones `origin`. It needs a network path to that remote.
- Set `VERIFY_INSTALL_TEST` only when a stub should stand in for this script. The Mac proof uses the real path.
