# Test from a clone

V8. "test the installation on the computer from a clone of the repository in its own workspace", with "a good way of testing it that isn't just actually waiting for something like a timeout of 10 minutes."

Run the helper from a separate clone on the owner's Mac. The helper passes `--idle-seconds 5`, so the proof does not wait out the 600 second default.

## Sub-features

- `v8-clone` runs the helper from a clone that is not the checkout you were already editing.
- `v8-short-idle` uses that helper's 5 second idle threshold.

## How to get to it (user POV)

- Clone the repository into a new directory.
- On the owner's Mac, run the helper from that directory.

## Driving it with bash

Preconditions:

- The machine is the owner's Mac. On any other OS the helper prints `FAIL doctor not macOS` and exits non-zero. That result does not pass V8.
- The binary is already built. `install` does not build. A missing binary is `FAIL`.

- **Clone.** Clone the repository into a new directory and `cd` there.
- **Run.** Run `.cursor/skills/verify-arch-caffeinate/scripts/verify.sh`.
- **Pass.** The helper exits 0. The transcript shows `--idle-seconds 5` on the install lines and `cleanup idleThresholdSeconds is 600`.
- **Proof.** Leave `artifacts/verify/<timestamp>/` from that clone in place.

## Gotchas

- A helper run in the checkout you are editing does not prove `v8-clone`.
- Waiting 600 seconds does not prove `v8-short-idle`. V1 through V7 passing from the separate clone is the V8 proof.
