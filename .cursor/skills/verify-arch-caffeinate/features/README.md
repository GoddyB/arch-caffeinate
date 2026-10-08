# arch-caffeinate verification map

This directory is the maintained source for verifying arch-caffeinate. Read this index, then the feature file that names the V-number you are proving. Each V-number is defined in `verbatim/2026-10-07-arch-caffeinate.md`.

## Baseline preconditions

- The machine is the owner's Mac. Any other OS stops after doctor reports `not macOS`.
- `arch-caffeinate` is installed per [docs/cli-contract.md](../../../../docs/cli-contract.md).
- The login shell for the proof is bash.
- One LaunchAgent (`io.github.goddyb.arch-caffeinate`) is loaded. Do not start a second `run`.
- `arch-caffeinate doctor` has no `FAIL` line before a Mac proof.
- The helper may pass `--idle-seconds 5`. Restore 600 seconds in cleanup.

## Driving conventions

- Start from the baseline unless the feature file says otherwise.
- Run commands in bash. Copy the command, the output, and the exit code into the evidence directory.
- Treat every command as literal.
- A skipped entry point stays unverified. Do not claim another path covered it.

## Proof and skip reporting

- Record the V-number with the command that proves it.
- Sleep prevention proof is `pmset -g assertions`, not only `sleepPrevented`. The count is at least 1, and the listing names the daemon's `caffeinate` child.
- Display proof is `CurrentPowerState` from `ioreg -r -d 1 -c AppleCLCD2`. `1` means on and `0` means off. `status --json` `display` must match that reading. V2 and V3 fail when the ioreg reading is missing. V2 also requires `HIDIdleTime` of at least 5 seconds.
- Idle proof is `HIDIdleTime` from `ioreg -c IOHIDSystem`. The value is nanoseconds.
- Screen-lock proof is the `sysadminctl` phrase `screenLock delay is off`, `status --json` field `screenLock` set to `off`, and the doctor line `PASS screenlock: off`.
- Shell proof is `bash -lc 'command -v arch-caffeinate'` and Ghostty `+show-config`.

## Features

- [Sleep on AC](./sleep-on-ac.md) proves V1.
- [Display off](./display-off.md) proves V2.
- [Wake](./wake.md) proves V3.
- [Unlocked wake](./unlocked-wake.md) proves V4.
- [Bash in Ghostty](./bash-in-ghostty.md) proves V5 and V6.
- [Install and status](./install-and-status.md) proves V7.
- [Install from clone](./install-from-clone.md) proves V8.
