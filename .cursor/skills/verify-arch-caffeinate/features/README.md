# arch-caffeinate verification map

This directory is the maintained source for verifying arch-caffeinate. Read this index, then the feature file that names the V-number you are proving. The words in quotes are the product spec subset in `verbatim/2026-10-07-arch-caffeinate.md`.

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
- Sleep prevention proof is `pmset -g assertions`, not only `sleepPrevented`.
- Display proof is `ioreg` or `pmset` power state for `IODisplayWrangler`.
- Idle proof is `HIDIdleTime` from `ioreg -c IOHIDSystem`.
- Screen-lock proof is `sysadminctl -screenLock status` next to `status --json`.
- Shell proof is `bash -lc 'command -v arch-caffeinate'` and Ghostty `+show-config`.

## Features

- [Sleep on AC](./sleep-on-ac.md) proves V1.
- [Display off](./display-off.md) proves V2.
- [Wake](./wake.md) proves V3.
- [Unlocked wake](./unlocked-wake.md) proves V4.
- [Bash in Ghostty](./bash-in-ghostty.md) proves V5 and V6.
- [Install and status](./install-and-status.md) proves V7.
