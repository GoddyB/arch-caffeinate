# Display off

After HID idle time reaches the threshold, the display is fully off.

## Sub-features

- `v2-off` turns the display fully off once the idle threshold is reached.
- `v2-once` does not repeat the off command while the display stays off.

## How to get to it (user POV)

- Leave the keyboard and mouse alone until the idle threshold passes.

## Driving it with bash

Preconditions:

- The helper installed with `--idle-seconds 5`, or you passed that flag yourself.
- Doctor has no `FAIL` line.

- **Read idle.** Run `ioreg -c IOHIDSystem` and record `HIDIdleTime`.
- **Wait.** Poll at most 8 seconds, one second at a time, until idle time is at least 5 seconds.
- **Read display.** Run `ioreg -r -d 1 -c AppleCLCD2`. `"CurrentPowerState"=0` means fully off. `1` means on. This is the full-off state `pmset displaysleepnow` produces, not the dim state. V2 fails when this reading is absent.
- **Read status.** Run `arch-caffeinate status --json`. `display` is `"off"`, the same reading as `CurrentPowerState`.
- **Proof.** Save the `ioreg` idle line and the display power listing together.

## Gotchas

- Do not use the 600 second default for this proof.
- A dimmed panel still counts as on. Require the full-off power state.
- Touching the keyboard during the wait resets `HIDIdleTime` and invalidates the run.
