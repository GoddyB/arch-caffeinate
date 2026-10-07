---
name: verify-arch-caffeinate
description: "Drive the installed arch-caffeinate macOS CLI from bash and prove sleep prevention, display power, wake, screen lock, Ghostty bash, and install. Use when verifying arch-caffeinate on the owner's Mac, or when doctor must report that the machine is not macOS."
---

# Verify arch-caffeinate

Drive the installed `arch-caffeinate` binary from a terminal running bash. The contract is [docs/cli-contract.md](../../../docs/cli-contract.md). The feature map is [features/README.md](features/README.md). One LaunchAgent label and one state file exist per login user. Do not start a second `arch-caffeinate run` while that agent is loaded.

Step 4 of create-verification-skill (prove end to end) runs on the owner's Mac after the CLI is installed. On any other machine, run only the helper. It prints `FAIL` with the reason.

## Launch

On the owner's Mac, from the clone, run the helper. It installs the already-built binary with `--idle-seconds 5`, then runs `arch-caffeinate doctor`.

```bash
.cursor/skills/verify-arch-caffeinate/scripts/verify.sh
```

Ready means `arch-caffeinate doctor` prints a `PASS` line for the LaunchAgent and a `PASS` line for the heartbeat. There is no server port. Each helper run uses the existing login agent. It does not open a second daemon.

## Doctor

Run `arch-caffeinate doctor` before any Drive step when a check looks wrong. Doctor is read-only. On a machine that is not macOS, the macOS line says `not macOS` and the command exits non-zero. Continue only when every doctor line is `PASS` or `WARN`.

## Drive

Read [features/README.md](features/README.md), then the feature file for the V-number you are proving. Run the commands in that file from bash. The helper runs every V-number in order with `--idle-seconds 5`.

Observable commands, copied into the evidence directory:

- `pmset -g assertions` shows a `PreventSystemSleep` assertion owned by the daemon while `power` is `ac`.
- `ioreg -n IODisplayWrangler -r -d 1` or `pmset -g powerstate IODisplayWrangler` shows the display power state.
- `ioreg -c IOHIDSystem` shows `HIDIdleTime`.
- `sysadminctl -screenLock status` shows the screen-lock setting.
- `bash -lc 'command -v arch-caffeinate'` prints the binary path.
- Ghostty `+show-config` shows the command Ghostty starts. Pass V6 only when that command is bash.

Use the 5 second threshold. Do not wait out the 600 second default. The helper bounds each wait at 8 seconds.

## Evidence

The helper writes `artifacts/verify/<UTC timestamp>/` in the clone. The directory holds `transcript.txt` (commands, stdout, stderr, exit codes) and `status.json` when the CLI answers. A proof shows the command and the next observed state. `status --json` alone does not prove sleep prevention or display power. Those proofs are the `pmset` and `ioreg` listings.

Do not change the screen-lock setting to make V4 pass. If it is not off, V4 is `FAIL` and the transcript quotes `sysadminctl`.

## Cleanup

The helper runs `arch-caffeinate install` with no idle flag so the LaunchAgent returns to the 600 second threshold. It leaves the binary, the plist, the log, and `artifacts/verify/<UTC timestamp>/` in place. It kills only a foreground `arch-caffeinate run` whose pid the helper started. It does not unload the restored agent.

## Helpers

`.cursor/skills/verify-arch-caffeinate/scripts/verify.sh` is the whole proof. It prints one `PASS` or `FAIL` line per V-number and exits 0 only when every line is `PASS`.

After the CLI changes, run `/maintain-verification-skill` so the feature map stays aligned with [docs/cli-contract.md](../../../docs/cli-contract.md).
