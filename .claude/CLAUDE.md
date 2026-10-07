# Claude Code in this repo

@../AGENTS.md

## GitHub identity: ArchAgents only

Claude Code does all work in this repo as the GitHub user `ArchAgents`. That covers comments, reviews, pull requests, issues, and API calls. Pushes have to go through gh's credential helper (`gh auth setup-git`). A push that uses git's own helper or SSH can be a different account, and the hook does not see that.

`.claude/hooks/require-archagents.sh` checks the identity on every tool call and at session start. The `*` matcher in `.claude/settings.json` is deliberate. `.claude/` demands this for all Claude Code work, including reads. If a hook reports a failure, stop. Don't switch accounts, borrow another token, or work around the check.

Cursor also runs these hooks. The script skips Cursor agents (`CURSOR_VERSION` set, `CLAUDECODE` unset), because the rule is for Claude Code.

Commits still use the owner's identity from AGENTS.md (`GoddyB <324734221+GoddyB@users.noreply.github.com>`). ArchAgents pushes and posts.
