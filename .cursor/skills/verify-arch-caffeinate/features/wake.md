# Wake

V3. "when I move the mouse or press a key, or even a notification pops up, the screen comes back on."

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

- **Declare activity.** Run `arch-caffeinate wake`. Within 3 seconds, `pmset -g powerstate IODisplayWrangler` shows the display on. `ioreg -c IOHIDSystem` shows `HIDIdleTime` reset.
- **Turn the display off again.** Repeat the display-off wait with `--idle-seconds 5`.
- **Deliver a notification.** Run `osascript -e 'display notification "verify" with title "arch-caffeinate"'`. Within 3 seconds the display power state is on.
- **Proof.** Save the off listing, the wake command, and the following on listing for each sub-feature.

## Gotchas

- `arch-caffeinate wake` is the non-interactive stand-in for a keypress. It has the same effect as `caffeinate -u -t 1`. A physical mouse move is optional when you are at the keyboard. Record it only if you perform it.
- A notification that never arrives does not prove `v3-notification`.
- Prove each sub-feature from a fully off display. Waking an already-on display proves nothing.
