# AGENTS.md

## Poteto mode on every turn

Every turn in this repo runs in poteto mode, and you make sure of it yourself. Don't count on the launching prompt to turn it on. At the start of each turn, check whether poteto mode is already active as a sticky mode (for example, `/poteto-mode` set it earlier in this session). If it is, carry on. If it isn't, invoke the pstack `poteto-mode` skill. Its model invocation is disabled, so read its `SKILL.md` and follow it.

## Source of truth

The owner's own words in `verbatim/` are the spec. Don't edit them. When code, docs, or tests disagree with `verbatim/`, the code is wrong.

## Public repo hygiene

This repo is public. Never commit an email address other than a GitHub noreply address, a home-directory path with a real username, a tailnet IP or hostname, a machine name, or a token. `scripts/check-public-hygiene.sh` enforces this in CI.

Commit as `GoddyB <324734221+GoddyB@users.noreply.github.com>`. Squash-merge with `gh pr merge <n> --squash --author-email 324734221+GoddyB@users.noreply.github.com`, because GitHub otherwise stamps the squash commit with the account's profile email.
