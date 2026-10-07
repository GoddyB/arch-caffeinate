# Bash in Ghostty

V5. "control this functionality from a CLI that is recognized when I use bash in Ghosty"

V6. "make it so that Ghosty always starts in bash"

Ghostty is the Ghosty terminal named in the spec. A login bash finds `arch-caffeinate`. Ghostty starts bash.

## Sub-features

- `v5-login-bash` resolves the binary from `bash -lc`.
- `v6-ghostty-bash` shows Ghostty's start command is bash.

## How to get to it (user POV)

- Open Ghostty. The shell is bash.
- Type `arch-caffeinate` and the shell finds the command.

## Driving it with bash

Preconditions:

- `install` has copied the binary to `~/.local/bin/arch-caffeinate`.
- Ghostty is installed on the owner's Mac.

- **Login bash.** Run `bash -lc 'command -v arch-caffeinate'`. The output is a path and the exit code is 0.
- **Ghostty config.** Run Ghostty's `+show-config`. Use `command -v ghostty` when it prints a path. Otherwise run `/Applications/Ghostty.app/Contents/MacOS/ghostty +show-config`.
- **Pass V6.** The config text has a `command` line whose value is bash (`bash` or `/bin/bash`). A zsh command is `FAIL`.
- **Proof.** Save both outputs under the evidence directory.

## Gotchas

- An interactive zsh that happens to see the binary does not prove V5. Use `bash -lc`.
- V6 fails closed when Ghostty is missing. Record the command you tried.
- Do not edit the Ghostty config inside this proof. The proof only reads `+show-config`.
