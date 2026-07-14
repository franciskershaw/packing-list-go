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

## 2026-07-10 — PACK-011 — Packing list management shipped; clean design, three process refinements

- No design rework — every endpoint matched the handoff doc's decisions on
  the first pass (all repo tests green first try, all four handler ACs
  green first try, zero regressions along the way).
- **Pattern**: stubs must fail in an assertable way (sentinel error / a
  real-but-wrong HTTP status), never a bare panic — Go's `testing.tRunner`
  recovers a panic, re-raises it, and kills the whole test binary, hiding
  every other test's result behind the first one. Codified in both
  CLAUDE.md files.
- **Pattern**: when the upper layer is tested entirely against a mock,
  implement and fully green the lower layer first — its own commit, its
  own review pause — before touching the upper layer. Zero real coupling
  cost once the interface is fixed, and reviews got noticeably cleaner
  split this way. Codified globally (step 5) plus a project-specific
  pointer.
- Manual-verification (`.http`) files need `{{$guid}}`-style unique names
  in any setup fixture with a per-user uniqueness check (category/template
  names) — a fixed name 409s against leftover state from an incomplete
  prior run, which then cascades (e.g. a failed category create leaves a
  downstream item create with no valid `categoryId`). Also moved to
  drafting each AC's `.http` section alongside its own implementation,
  with an explicit manual-check gate per commit, instead of writing the
  whole file upfront and only running it once at the end.

## 2026-07-10 — PACK-012 — Item management on packing lists shipped; one self-caught test bug

- Repo layer and 3 of 4 handler ACs (Add/Remove/BulkAdd) matched the
  handoff doc on the first pass. One real bug: a tests-first test for
  `UpdateItem` assumed body-validation-before-ownership, copied from
  PACK-011's own list-level `Update` rather than the actual named
  precedent (`TemplateHandler.UpdateItem`, ownership-first) — caught
  mid-implementation, not by review, and fixed by correcting the test.
- **Pattern**: when a test is written to mirror a named precedent, verify
  the exact call order against that precedent's current source at write
  time — not memory, not a similar-looking neighbor — and cite it in the
  test. Codified globally (step 4).
- **Pattern**: each planned test should trace to a specific acceptance
  criterion, or say explicitly that it guards a documented decision rather
  than covering new behavior (e.g. the `SucceedsOnArchivedList` tests this
  ticket, which are mock-identical to their plain-success siblings).
  Codified globally (step 3 handoff docs, and folded into the periodic
  tech-debt pass).
- `.http` sections are now self-contained (own fixtures, own cleanup) with
  an explicit "new in this commit" marker, so a single AC can be spot-
  checked without re-running everything above it. Applied from the second
  AC onward; the first section and all of PACK-010/011's stay as-is —
  full `.http` file structure is its own tech-debt item (PACK-014 #12),
  deliberately deferred until every feature ticket is done.

## 2026-07-10 — PACK-013 — Packing/ticking shipped; smooth ticket, one new stub-phase judgment call

- No rework — repo layer and both handler ACs (`PackAll`/`UnpackAll`) went
  green first try. Mechanical ripple from extending `UpdatePackingListItem`'s
  signature (adding `isPacked`) was handled cleanly, likely because
  PACK-012 had just been through the same class of change.
- This ticket extended an already-implemented method rather than stubbing a
  brand-new one — the existing "stubs return an error/wrong status, never
  panic" rule assumed brand-new methods. Judgment call made in-session:
  leave the real persistence unimplemented in the repo (the genuine stub),
  but do the forced mechanical handler-side passthrough immediately, since
  Go's type system requires every call site to supply the new argument
  regardless and that wiring isn't business logic. Not promoted to a rule
  this time — noted here in case the pattern recurs.
- `.http`: first ticket to extend an *existing* self-contained section
  (inserting new cases before its own cleanup) rather than only ever
  creating fresh ones.

## 2026-07-10 — PACK-014 — Security hardening shipped; handoff-doc archive near-miss caught in time

- No rework — both ACs (refresh-cookie `Secure`/`SameSite`, CSRF
  `crypto/rand`) matched the interview's design on the first
  implementation pass, tests green.
- **Pattern**: when a ticket was split out of a bundled `grill-me`
  findings archive, check whether that archive is still parked at the new
  ticket's own handoff-doc filename before writing over it — "handoff doc
  already exists" doesn't always mean "this ticket's own prior draft."
  Here `docs/handoffs/PACK-014.md` still held items 3–12 that
  PACK-015–020 need; caught before writing, archive moved to
  `docs/handoffs/epic-6-findings.md`.
- Also worth noting: adding a new field to an existing handler's
  dependency (`cfg.Environment`) surfaced 8 test call sites silently
  passing `nil` for it — a quick grep for existing constructor callers
  before assuming a new field is additive-only paid off.

## 2026-07-10 — PACK-015 — Config threading shipped; large ripple, urgency lost in translation

- No rework — both ACs (`db.InitDB` parameter threading,
  `jwt.go` secret threading + Access/Refresh consolidation) matched the
  interview's design and went green on the first pass. But the diff
  touched 15 files — Go forces every caller of a changed signature to
  update in lockstep, and the in-pass duplication cleanup (agreed in the
  interview) added further scope.
- **Pattern**: the usual stub-then-red-then-implement TDD flow doesn't
  fit a ticket that changes existing function signatures with call sites
  scattered across the codebase — there's no way to get a compiling test
  suite without wiring the real logic everywhere at once. Confirmed and
  named explicitly this ticket (extends the PACK-013 "mechanical
  passthrough isn't business logic" judgment call to full-ticket scale) —
  flag this up front rather than manufacturing an artificial/flaky red
  state.
- **Pattern**: at close-out, re-checking the source finding's original
  urgency tier revealed this was archived as "worth knowing about — real,
  but not urgent," not one of the two items with actual security
  relevance (those were PACK-014's). The user had lost track of that
  distinction by the time PACK-015 was picked up. When `grill-me` starts
  a ticket sourced from a findings/archive doc, restate that tier as part
  of the opening context so the urgency call is made *before*
  implementation, not rediscovered after.
- Also noted: I went quiet too long during blast-radius exploration on a
  wide-ripple ticket and the user had to ask what was happening — give an
  interim status update before a long silent tool-call sequence,
  especially once a ticket's blast radius is visibly large.

## 2026-07-10 — PACK-016 — user.go not-found convention shipped; test-reporting gap found and fixed

- No implementation rework — both ACs (repo `nil, nil` convention,
  `RefreshToken` 401-vs-500 split) matched the interview's design and
  went green on the first pass.
- Surfaced a real reporting gap: `internal/repository/...` tests had been
  silently self-skipping all session (no `DATABASE_URL` in Claude's shell)
  without that being flagged clearly enough — the user assumed they'd
  been running against the real Neon DB in earlier tickets. Resolved with
  a standing split (user runs the DB-touching suite as final check;
  Claude drafts/updates it and confirms everything else green) and a
  habit of always naming a skip explicitly rather than letting "tests
  pass" imply full coverage.
- **Pattern**: at the close-out gate, give copy-pasteable verification
  commands (pulled from the handoff doc + project CLAUDE.md) alongside
  the yes/no question instead of asking blind. Promoted live this ticket
  by editing the `close-out` skill's Step 2 directly.
- One test assertion flipped (`Error` → `NoError` in
  `TestGetUserByID_NotFound`) purely as a mechanical consequence of the
  intentional contract change (not-found now `nil, nil`) — not a fix to a
  previously-wrong test. Worth restating in a retro when a diff includes
  what looks like a reversed assertion, so it doesn't read as a hidden
  bug fix.

## 2026-07-10 — PACK-017 — OAuth test isolation shipped; clean ticket

- No rework. Both ACs (network-free test construction via an unexported
  `newGoogleOAuthManager` constructor, `contains()` → `strings.Contains`)
  matched the interview's design and went green on the first pass.
- Verified test isolation directly this time (ran the suite with a
  broken `HTTP_PROXY`/`HTTPS_PROXY` to force any accidental network call
  to fail fast) rather than just trusting the refactor — worth doing
  again whenever a ticket's whole point is removing an I/O dependency
  from tests.

## 2026-07-10 — PACK-018 — Naming & duplication cleanup shipped; the DATABASE_URL skip was a false-positive gate all session

- No implementation rework — all 4 ACs (casing, the 13-site `bindJSON`
  consolidation across 6 files, `go.mod tidy`, notes-helper rename)
  matched the interview's design and went green first pass. The
  `parseName` finding itself didn't hold up as archived — investigated
  first, found the real duplication was 4x bigger (13 sites/6 files, not
  3) than originally scoped, fixed all of it rather than the narrow
  version.
- **The real story**: `DATABASE_URL=$DATABASE_URL go test
  ./internal/repository/...` silently expands to empty and skips in the
  user's own shell, not just Claude's sandboxed one — meaning PACK-016's
  "ran repo tests locally, they pass" (and possibly earlier) never
  actually ran against Neon. Caught by noticing 107 tests completing in
  0.25s was implausible (real run: ~27.6s), not by reading the output,
  since a silent skip's "ok" looks identical to a real pass without `-v`.
- **Pattern**: when a test run's plausibility is in doubt (timing too
  fast, a skip path exists, etc.), ask for `-v` output and actually look
  at it — individual test names or an explicit skip message — rather
  than trusting a bare "ok". Don't assume a passing summary line means
  what it appears to mean.
- User reconfirmed (now for two reasons — honesty about who verifies,
  and the ~27.6s cost) that they run `internal/repository/...` against
  Neon themselves as a deliberate TDD step, with the real command:
  `set -a; source .env; set +a; go test ./internal/repository/... -v
  -count=1`.

## 2026-07-10 — PACK-019 — doRequest retrofit shipped; clean ticket

- No rework. All 5 ACs (167 call sites across 5 files, including a 5th
  file — `packing_list_handler_test.go` — the archived finding wrongly
  assumed already used `doRequest`) matched the interview's design, went
  green first pass.
- **Pattern**: for a large mechanical retrofit (same change repeated many
  times), a scripted transformation + before/after test-name-list diffing
  per file is a fast, safe verification method — reuse it next time a
  ticket is "the same edit repeated N times."

## 2026-07-11 — PACK-020 — requests/*.http overhaul shipped; deliberately reversed the PACK-012 convention

- No rework — all 7 ACs matched the interview's design and passed manual
  verification first pass. Scope got reframed mid-interview: item 12's
  original "smoke vs. regression split" framing was dropped once it was
  clear per-commit isolation isn't the priority now that every feature
  ticket is done — that reframe drove most of the design.
- **Pattern**: when a convention was adopted to solve a specific problem
  (PACK-012's self-contained `.http` sections, for per-commit isolation)
  and that problem stops applying, reversing the convention is fine — say
  so explicitly in the handoff/retro rather than let it read as drift, so
  the log keeps the "why" for both the adoption and the reversal.
- Found live JWTs committed in git history (`templates.http`,
  `template_items.http`) incidentally while reading files to plan the
  rewrite — expired/dev-only, no remediation needed, but it's the exact
  class of mistake the `@token`→`{{$dotenv}}` fix now prevents
  structurally rather than just discourages.
- Verified the structural rewrite by diffing request/method counts against
  git history (`packing_lists.http`: 128→79) rather than only reading the
  new file — same before/after-diff technique as PACK-019's mechanical
  retrofit, now shown to generalize to a structural rewrite, not just a
  repeated-edit retrofit.

## 2026-07-14 — PACK-029 — Unarchive endpoint shipped; clean ticket

- No rework — design followed precedent throughout (`PackAll`/`UnpackAll`'s
  action-verb-pair route shape, `ArchivePackingList`'s idempotent-UPDATE
  style, the newer `requireOwnedPackingList` helper). Stubs failed red for
  the right reason (5/6 new handler tests), real implementation went green
  first pass on both the mocked handler suite and the Neon-backed repo
  tests.
- One grill-me question offered two file-placement options that both
  assumed a dedicated "archive handler" file already existed to mirror —
  it doesn't, archive has always lived in `packing_list_handler.go`. User
  caught it and answered with a clarifying question instead of picking
  either option. Not promoted to a pattern — one-off, not worth a
  standing rule.
