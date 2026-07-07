# Lessons

Running retro log for this repo. One entry per ticket close-out: what
caused rework (if anything), what pattern should become a standing rule,
and whether this file or the project's own `CLAUDE.md` needed a new line
as a result. Reviewed at the start of every new ticket's `grill-me` and at
project kickoff.

## 2026-07-07 — Kickoff retrofit

- Ran `project-kickoff` retroactively, after PACK-001 through PACK-007 were
  already built and PACK-008 (templates) was half-built without a handoff
  doc or tests. That half-built state (model + repo stubs, no handler, no
  tests, nothing wired into `main.go`) is what surfaced the need for this
  process in the first place.
- **Pattern to keep**: PACK-006/007 (categories, items) show the process
  working — git history has the test-file commit landing before the
  handler commit in both cases, even without a formal handoff doc existing
  yet at the time. Tests-first doesn't require heavyweight process to
  happen; it requires discipline in commit order.
- **Standing rule going forward**: no ticket gets implementation code until
  it has a `docs/handoffs/PACK-NNN.md` and a failing test suite, per
  `~/.claude/CLAUDE.md`. PACK-008 is being fully redone under this rule.
