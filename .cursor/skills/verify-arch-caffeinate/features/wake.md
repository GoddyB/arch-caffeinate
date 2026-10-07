# Wake

V3. "wakes on mouse move, keypress, or notification"

Mouse movement, a keypress, or a delivered notification turns the display on.

## Sub-features

- `v3-input` wakes the display when user activity is declared.
- `v3-notification` wakes the display after a delivered notification.

## How to get to it (user POV)

- Move the mouse, press a key, or receive a notification while the display is fully off.

## Driving it with bash

Preconditions:

- [Display off](./display-off.md) has just left the display fully off.
- Doctor has no `FAIL` line.

- **Simulated user activity.** Run `arch-caffeinate wake`. A remote run cannot move the mouse, so this command stands in for that activity. Within 3 seconds, `ioreg -r -d 1 -c AppleCLCD2` shows `"CurrentPowerState"=1`. `status --json` `display` is `"on"`. `ioreg -c IOHIDSystem` shows `HIDIdleTime` reset. V3 fails when the AppleCLCD2 reading is absent.
- **Turn the display off again.** Repeat the display-off wait with `--idle-seconds 5`.
- **Deliver a notification.** Run `osascript -e 'display notification "verify" with title "arch-caffeinate"'`. Within 3 seconds `ioreg -r -d 1 -c AppleCLCD2` shows `"CurrentPowerState"=1` and `status --json` `display` is `"on"`.
- **Proof.** Save the off listing, the wake command, and the following on listing for each sub-feature.

## Gotchas

- `arch-caffeinate wake` is simulated user activity. It has the same effect as `caffeinate -u -t 1`. A remote run cannot move the mouse. Record a physical mouse move only if you perform it.
- A notification that never arrives does not prove `v3-notification`.
- Prove each sub-feature from a fully off display. Waking an already-on display proves nothing.
