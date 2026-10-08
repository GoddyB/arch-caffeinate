# Unlocked wake

The display wakes without a password prompt only when the macOS screen-lock setting is off. `status` and `doctor` report that setting.

## Sub-features

- `v4-setting` reports screen lock as off from `sysadminctl` and from `status --json`.
- `v4-doctor` reports the same setting from `arch-caffeinate doctor`.

## How to get to it (user POV)

- Wake the display and see the desktop, with no password field.

## Driving it with bash

Preconditions:

- You can run `sysadminctl -screenLock status`.
- Do not change the setting during the proof.

- **Read the setting.** Run `sysadminctl -screenLock status`. The setting text is on stderr. Pass only when the last phrase `screenLock delay is immediate`, `screenLock delay is off`, or `screenLock delay is <seconds>` is `off`. A usage line that only contains the word `off` does not pass.
- **Read status.** Run `arch-caffeinate status --json`. `screenLock` is `"off"`.
- **Read doctor.** The doctor transcript includes the line `PASS screenlock: off`.
- **Proof.** Save both command outputs. If the setting is immediate or a delay, V4 is `FAIL`. Quote the `sysadminctl` line in the reason.

## Gotchas

- Do not turn screen lock off to force a pass. The owner's Mac is expected to already be unlocked on wake.
- `screenLock` of `"unknown"` is `FAIL`.
- A password dialog cannot be accepted by this helper. The setting is the observable proof.
