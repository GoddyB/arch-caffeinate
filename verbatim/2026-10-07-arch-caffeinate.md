# arch-caffeinate instructions, 2026-10-07

Source: the owner's phone call on 2026-10-07 at 3:18 PM Pacific. The call's word-for-word transcript was not saved when this file was written, so the text below is the call relay's record of the instructions, kept unchanged. If the word-for-word transcript turns up, it replaces this text in a follow-up change.

Omitted: nothing personal appeared in the relay.

> Build CLI "arch-caffeinate", public repo under GitHub account Goddy B (not gdsrvnt). Constraints: (1) Repo named arch-caffeinate, public, no personal info leaked, workspace for cursor cloud agents. (2) macOS tool: prevents sleep while plugged in; after inactivity turns screen fully off (not macOS dim state) to prevent burn-in; wakes on mouse move, keypress, or notification — like a phone lock screen but unlocked. (3) CLI recognized in bash in Ghosty; make Ghosty always start in bash (not zsh). (4) First: create verification skill using Potato mode; spec = subset of my prose instructions as source of truth, in folder "verbatim". (5) All repo work uses Grok 4.7 low fast mode; if cloud agent usage runs out, use Py tool with GPT 6.1 Sol from scoped models of my Py install, in its own workspace via use-herder skill. (6) Test suite must not rely on long timeouts (e.g. 10 min); include testing install from a repo clone in its own workspace. (7) Scan test suite with Opus 5.5 for tautological/empty-calorie tests. (8) Follow P stack playbooks closely; at least one playbook at top of every cloud agent prompt. (9) Once built, install on my computer; I am first user; test it. (10) Text me ONLY when CLI is fully built and installed/running on my laptop. No other communication except: blocked → single sentence or credential/multiple-choice card (emergency only); requests for me to take over computer; status updates under one sentence; final green emoji "done". (11) Activate Potato mode, create task list from this message early, then begin. Do not wait on me; no PR approvals needed.

## Product spec subset

These are the sentences above that describe what the tool must do. The verification skill checks the installed CLI against exactly these.

- V1. "macOS tool: prevents sleep while plugged in"
- V2. "after inactivity turns screen fully off (not macOS dim state) to prevent burn-in"
- V3. "wakes on mouse move, keypress, or notification"
- V4. "like a phone lock screen but unlocked"
- V5. "CLI recognized in bash in Ghosty"
- V6. "make Ghosty always start in bash (not zsh)"
- V7. "Once built, install on my computer; I am first user; test it."

Glossary for readers: "Ghosty" is the Ghostty terminal. "Potato mode" is pstack's poteto mode. "P stack" is pstack. "Py" is the Pi coding agent. "herder" is herdr.
