# Sleep on AC

V1. "macOS tool: prevents sleep while plugged in"

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

- **Read assertions.** Run `pmset -g assertions`. The listing contains `PreventSystemSleep` and the daemon pid from `status --json`.
- **Read status.** Run `arch-caffeinate status --json`. `sleepPrevented` is `true` only in that same moment.
- **Proof.** Save both outputs under `artifacts/verify/<timestamp>/`. The assertion listing is the proof.

## Gotchas

- `caffeinate` from another program can add its own assertion. Match the daemon pid.
- A `status` bool without the `pmset` listing is not V1.
- Battery fails V1 for this run. It does not prove the release path.
