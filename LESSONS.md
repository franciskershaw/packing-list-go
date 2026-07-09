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

## 2026-07-08 — PACK-008 — Template CRUD shipped; self-review gap surfaced

- CLAUDE.md's TDD guidance was unclear going in (fixed mid-ticket with an
  explicit Testing section + Go stub-then-red-then-green rule). Separately,
  obvious handler-level duplication (a repeated validation block) and a
  misplaced helper (entity-specific vs. shared file) slipped through and
  were only caught on the user's manual review, not flagged by me.
- **Pattern**: After writing new handler/repo code, do a quick self-scan for
  duplicated logic blocks and helper file-placement before presenting the
  work as done — don't rely on the user's review or the once-per-ticket
  `/code-review` to catch routine quality issues like this.
- No other rework. The CLAUDE.md gap was resolved in-session, not carried
  forward.

## 2026-07-09 — PACK-009 — Item-on-template endpoints shipped; pacing violation, not a design one

- Implementation itself was clean (no design rework), but I ran straight
  through test-writing → stubs → full implementation of all four endpoints
  in one uninterrupted pass, instead of stopping after the test suite for
  review. I also wrote `requests/template_items.http` late (alongside
  routing/wiring) rather than alongside the unit tests. Both are explicit
  in CLAUDE.md's pipeline; I misread "stop at commit boundaries" as tied to
  the *act of committing* rather than as an independent signal to hand back
  control, and since this user owns all commits I never hit a natural
  trigger to pause.
- **Pattern**: Tests (repo + handler) and the `.http` manual verification
  file are one deliverable, written together, before any stub or
  implementation code. Stop there and wait for explicit review — "let's
  go" said about the ticket overall does not authorize skipping this
  checkpoint.
- No design rework — the repository/handler split, ownership checks, and
  validation all matched the handoff doc on the first pass.

## 2026-07-09 — PACK-010 — Packing list creation shipped; smooth ticket

- No rework worth noting. The test-first pause (per the PACK-009 lesson
  above) held this time. Only wrinkle was `requests/packing_lists.http`
  asserting 204 on cleanup deletes that a later request's own side effect
  (seeding `packing_list_items` from a template) actually blocks with 409 —
  caught and fixed post-review, not a code bug.
