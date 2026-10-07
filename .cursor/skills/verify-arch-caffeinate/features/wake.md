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

- **Wake coverage.** The display must already be off. Run `arch-caffeinate wake`. That command stands in for mouse or key input. Within 3 seconds, `ioreg -r -d 1 -c AppleCLCD2` shows `"CurrentPowerState"=1` and `status --json` `display` is `"on"`. The PASS line says `wake` is the stand-in.
- **Notification while stopped.** Run `arch-caffeinate stop`, then `osascript -e 'display notification "verify" with title "arch-caffeinate"'`. For 3 samples, `CurrentPowerState` stays `0`.
- **Notification after start.** Run `arch-caffeinate start`, post the same notification, and expect `CurrentPowerState` `1`.
- **Proof.** Save the off listing, the wake command, and the following on listing for each sub-feature.

## Gotchas

- `arch-caffeinate wake` is simulated user activity. It has the same effect as `caffeinate -u -t 1`. A remote run cannot move the mouse. Record a physical mouse move only if you perform it.
- A notification that never arrives does not prove `v3-notification`.
- How long a notification keeps the display on is a manual gap. This check only separates a stopped agent from a started one. It does not measure whether the next poll turns the display off again.
- Prove each sub-feature from a fully off display. Waking an already-on display proves nothing.
