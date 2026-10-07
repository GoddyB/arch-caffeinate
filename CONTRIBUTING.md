# Contributing

Run the tests from a clone.

```bash
go test ./...
bash scripts/test-install-from-clone.sh
bash scripts/check-public-hygiene.sh
```

`go test` uses an injected clock and a fake macOS adapter. No test waits more than a second. `scripts/test-install-from-clone.sh` clones the repo into a temp directory, builds the binary, and runs `install` with `HOME` and `ARCH_CAFFEINATE_LAUNCHCTL` pointed at a stub.

`scripts/check-public-hygiene.sh` fails the commit when a tracked file contains an email address other than a GitHub noreply address, a home-directory path with a real username, a tailnet IP or hostname, a machine name, or a token.

Commit as `GoddyB <324734221+GoddyB@users.noreply.github.com>`.

`verbatim/` is the owner's spec. Read it. Do not edit it. When code, docs, or tests disagree with `verbatim/`, the code is wrong.
