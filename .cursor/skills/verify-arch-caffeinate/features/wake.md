# Wake

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

- **Wake coverage.** The display must already be off. Capture `HIDIdleTime` from `ioreg -c IOHIDSystem`. Run `arch-caffeinate wake`. That command stands in for mouse or key input. The next `HIDIdleTime` is lower, and for 3 samples `ioreg -r -d 1 -c AppleCLCD2` shows `"CurrentPowerState"=1` and `status --json` `display` is `"on"`. The PASS line says `wake` is the stand-in.
- **Off again.** Leave input alone. Within 8 seconds, `HIDIdleTime` is at least 5 seconds and `CurrentPowerState` is `0` again. `startup remove` does not turn the panel off.
- **Turn the display off again.** Run `arch-caffeinate startup remove`, then `pmset displaysleepnow`. Within 3 seconds `CurrentPowerState` is `0`.
- **Notification while unloaded.** Post `osascript -e 'display notification "verify" with title "arch-caffeinate"`. For 3 samples, `CurrentPowerState` stays `0`.
- **Notification after startup install.** Run `arch-caffeinate startup install`. Wait until `status --json` reports `"running": true` and `state.json` has a new pid with a fresh `writtenAt`. Then post the same notification, and expect `CurrentPowerState` `1`.
- **Proof.** Save the off listing, the wake command, and the following on listing for each sub-feature.

## Gotchas

- `arch-caffeinate wake` is simulated user activity (`caffeinate -u -t 1`); a remote run cannot move the mouse. Record a physical mouse move only if you perform it.
- A notification that never arrives does not prove `v3-notification`.
- `HIDIdleTime` must drop across `wake`. That drop is what keeps the next poll from turning the display off again.
- Prove each sub-feature from a fully off display. Waking an already-on display proves nothing.
