# Unlocked wake

V4. "So almost like a phone lock screen but not actually locked."

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

- **Read the setting.** Run `sysadminctl -screenLock status`. Pass only when the text says the setting is off.
- **Read status.** Run `arch-caffeinate status --json`. `screenLock` is `"off"`.
- **Read doctor.** Run `arch-caffeinate doctor`. The screen-lock line is `PASS` and agrees with `sysadminctl`.
- **Proof.** Save both command outputs. If the setting is immediate or a delay, V4 is `FAIL`. Quote the `sysadminctl` line in the reason.

## Gotchas

- Do not turn screen lock off to force a pass. The owner's Mac is expected to already be unlocked on wake.
- `screenLock` of `"unknown"` is `FAIL`.
- A password dialog cannot be accepted by this helper. The setting is the observable proof.
