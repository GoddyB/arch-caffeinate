# AGENTS.md

## Poteto mode on every turn

Every turn in this repo runs in poteto mode, and you make sure of it yourself. Don't count on the launching prompt to turn it on. At the start of each turn, check whether poteto mode is already active as a sticky mode (for example, `/poteto-mode` set it earlier in this session). If it is, carry on. If it isn't, invoke the pstack `poteto-mode` skill. Its model invocation is disabled, so read its `SKILL.md` and follow it.

## Source of truth

The owner's own words in `verbatim/` are the spec. Don't edit them. When code, docs, or tests disagree with `verbatim/`, the code is wrong.

## Public repo hygiene

This repo is public. Never commit an email address other than an allowed commit email, a home-directory path with a real username, a tailnet IP or hostname, a machine name, or a token. `scripts/check-public-hygiene.sh` enforces this in CI.

Allowed commit emails are addresses that end in `users.noreply.github.com`, the address `noreply@github.com`, and the address `cursoragent@cursor.com`. Commits listed in `.hygiene-known-leaks` print a warning and do not fail the check.

Commit as `GoddyB <324734221+GoddyB@users.noreply.github.com>`. Don't merge PRs with GitHub's merge button or `gh pr merge`. GitHub stamps those merge commits with the account's profile email. The orchestrator fast-forwards `main` to a green PR head instead.
