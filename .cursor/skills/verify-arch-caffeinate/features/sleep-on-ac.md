# Sleep on AC

While the Mac is on AC power, the daemon holds a system-sleep assertion. On battery it releases that assertion.

## Sub-features

- `v1-assert` holds `PreventSystemSleep` on AC power.
- `v1-release` drops that assertion on battery. Skip this line when the Mac is on AC and record the skip.

## How to get to it (user POV)

- Plug the Mac into power.
- Leave `arch-caffeinate` installed and the LaunchAgent loaded.

## Driving it with bash

Preconditions:

- `arch-caffeinate doctor` has no `FAIL` line.
- `arch-caffeinate status --json` reports `"power": "ac"`. If `power` is `battery`, V1 is `FAIL` with that value. Do not pretend AC.

- **Read assertions.** Run `pmset -g assertions`. Pass only when a line matches `PreventSystemSleep` with a count of 1, and a `pid N(caffeinate):` line names a child of the daemon pid from `status --json` (`pgrep -P <pid> -x caffeinate`).
- **Read status.** Run `arch-caffeinate status --json`. `sleepPrevented` is `true` in that same moment.
- **Proof.** Save both outputs under `artifacts/verify/<timestamp>/`. The assertion listing is the proof.

## Gotchas

- `caffeinate` from another program can add its own assertion. Match the daemon pid.
- A `status` bool without the `pmset` listing is not V1.
- Battery fails V1 for this run. Releasing the assertion on battery is a manual gap. A remote session cannot unplug the Mac.
