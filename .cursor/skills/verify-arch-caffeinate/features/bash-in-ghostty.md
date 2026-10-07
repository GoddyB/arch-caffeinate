# Bash in Ghostty

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

- **Login bash.** `bash -lc 'command -v arch-caffeinate'` equals `$HOME/.local/bin/arch-caffeinate`, and `bash -lc 'arch-caffeinate --version'` equals `"$BIN" --version`.
- **Ghostty config.** Run Ghostty's `+show-config`. Use `command -v ghostty` when it prints a path. Otherwise run `/Applications/Ghostty.app/Contents/MacOS/ghostty +show-config`.
- **Pass V6.** A line matches `^command = (/bin/|/opt/homebrew/bin/)?bash( .*)?$`. `initial-command = bash` does not pass. A zsh `command =` line is `FAIL`.
- **Proof.** Save both outputs under the evidence directory.

## Gotchas

- An interactive zsh that happens to see the binary does not prove V5. Use `bash -lc`.
- V6 fails closed when Ghostty is missing. Record the command you tried.
- Do not edit the Ghostty config inside this proof. The proof only reads `+show-config`. Nothing in this repo writes that config.
- `install` does not put `~/.local/bin` on `PATH`. V5 on a fresh account stays a manual gap until the login profile already contains that directory.
