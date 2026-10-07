# Claude Code in this repo

@../AGENTS.md

## GitHub identity: ArchAgents only

Claude Code does all work in this repo as the GitHub user `ArchAgents`. That covers comments, reviews, pushes, pull requests, issues, and API calls.

Before any work, check `gh api user --jq .login`, which honors `GH_TOKEN`. If it isn't `ArchAgents`, stop and report it. Don't switch accounts, borrow another token, or work around the check.

`.claude/settings.json` enforces the rule with `.claude/hooks/require-archagents.sh`:
- A `PreToolUse` hook blocks every tool call unless the login is `ArchAgents`.
- A `SessionStart` hook reports the result.
- `gh auth switch`, `gh auth login`, and `gh auth token` are denied.
- Cursor also runs these hooks. The script skips Cursor agents (`CURSOR_VERSION` set, `CLAUDECODE` unset), because the rule is for Claude Code.

Commits still use the owner's identity from AGENTS.md (`GoddyB <324734221+GoddyB@users.noreply.github.com>`). ArchAgents only pushes and posts.
