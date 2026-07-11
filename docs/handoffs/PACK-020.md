# PACK-020 — `requests/*.http` structural rethink

One-line summary: rebuild the manual `.http` test suite (all 5 existing
files + the token-generation tooling) into one consistent, low-duplication
convention now that every feature ticket touching it (Epics 1-5) is done —
this is the final task of the current development phase.

## Context

Deferred since PACK-012 as item 12 in `docs/handoffs/epic-6-findings.md`
("Not a one-line fix — flagged specifically so it isn't rushed or bundled
into an unrelated ticket"). Now unblocked. Full background/rationale for
why this was deferred lives in that file; this doc captures the interview
that resolved it.

**Key decisions from the interview (2026-07-11):**

- **Scope reframed from the original finding.** Item 12 asked whether one
  file can serve both "quick per-commit spot check" and "full manual
  regression pass." With every feature ticket now done, the per-commit
  fast-iteration need is deprioritized — the actual goal right now is one
  clean, comprehensive regression suite to run through as this phase
  closes out. No smoke-test file/section split is being built. The idea of
  a future *temporary, uncommitted* `.http` file for isolated testing of a
  hypothetical future feature was raised but explicitly parked, not
  scoped — revisit if/when that need is concrete.

- **This reverses the PACK-012 lesson.** PACK-012 established that new
  sections should be self-contained (own setup/cleanup, no reliance on
  earlier sections), specifically to make per-commit isolation easier —
  see the "👉 NEW IN THIS COMMIT" sections in the current
  `packing_lists.http`. Since per-commit isolation is no longer the
  priority, this ticket **reverts to chaining sections off shared
  `@name`-captured setup within a file** (the style already used
  throughout `categories.http`, `items.http`, `templates.http`, and
  `template_items.http`) — less duplication, reads as one coherent
  top-to-bottom pass. This is a deliberate, explicit reversal, not scope
  drift; call it out again at close-out so `LESSONS.md` records why the
  PACK-012 convention didn't stick.

- **Token generation is being automated.** Today, `go run
  scripts/gen_token.go` prints a token that gets manually pasted into a
  separate `@token` variable in *each* of the 5 files — real friction, and
  a real footgun: `templates.http` and `template_items.http` currently
  have **live JWT values committed in git history** in plaintext (both
  now expired — dev-only tokens signed with the local `JWT_SECRET_ACCESS`
  against a placeholder dev user, so no rotation/action needed, just
  noted). Fix: `gen_token.go` upserts the token into `.env` as `DEV_TOKEN`
  (already-gitignored, already read by this script), and every `.http`
  file references it via VS Code REST Client's `{{$dotenv DEV_TOKEN}}`
  instead of a pasted `@token`. This structurally prevents the
  commit-a-real-token class of mistake going forward.

- **Host is also pulled from `.env`**, reusing the *existing* `PORT` var
  (already read by `config.Config` / `main.go`) rather than introducing a
  parallel `API_HOST` that could drift: requests use
  `http://localhost:{{$dotenv PORT}}` inline instead of a per-file `@host`
  variable.

- **`requests/auth.http` is explicitly out of scope.** The browser-based
  Google login/callback can't be driven by a plain `.http` request (no
  browser rendering) — building this properly is its own piece of work,
  already flagged in `epic-6-findings.md`'s note under item 12
  (2026-07-10, PACK-014 grill-me). This ticket only restructures the 5
  existing resource files.

- **401 "no token" checks are being trimmed.** Today every single
  endpoint (GET/POST/PATCH/DELETE) gets its own repeated 401 check — real,
  visible duplication (4 near-identical checks in `categories.http`
  alone). Since the auth middleware is shared/global, each file keeps only
  1-2 representative 401 checks (one read, one write) rather than one per
  route. No other duplication was flagged as a concern — per-resource
  validation checks (400s for missing/empty/too-long name, etc.) test
  genuinely different resources/rules and stay as-is.

- **Cleanup consolidates into one block per file**, at the end, deleting
  everything the file created in reverse dependency order — replacing the
  current per-section repeated cleanup in `packing_lists.http` (the only
  file using the self-contained style being reverted here). Where a
  DELETE endpoint's own happy-path test already removes a resource (e.g.
  `categories.http`'s own `DELETE /categories/:id` test), that's not
  duplicated in the cleanup block.

- **A short `requests/README.md`** documents the convention once
  (token/host via `.env`, run top-to-bottom, cleanup expectations) as the
  canonical reference, rather than relying solely on each file's own
  SETUP header comment (which stays, but stays terse).

- **`gen_token.go` gets no automated test.** It's a `//go:build ignore`
  dev script, excluded from the normal build/test suite, with no existing
  test precedent — verified manually by running it and checking `.env`,
  which happens naturally as part of using the new `.http` files.

- **Commit granularity** (explicit override of the usual
  one-commit-per-AC default, confirmed by the user in the interview):
  1. `scripts/gen_token.go` + `.env.example` change, alone.
  2. All 5 `.http` files rewritten to the new convention + the new
     `requests/README.md`, together, in one commit.
  Pause for review after each.

## Acceptance criteria

- [ ] **AC1 — Token tooling.** `scripts/gen_token.go` upserts a
      `DEV_TOKEN=<token>` line into `.env` (creating/replacing just that
      line, not clobbering other vars) in addition to its existing stdout
      print. `.env.example` gets a `DEV_TOKEN=` placeholder line added.
- [ ] **AC2 — Consistent token/host mechanism across all 5 files.** No
      file has a manually-pasted `@token` or a per-file `@host` variable.
      Every request's `Authorization` header uses `{{$dotenv DEV_TOKEN}}`;
      every request URL uses `http://localhost:{{$dotenv PORT}}` inline.
      Each file's SETUP comment is updated to describe the new one-step
      flow (`go run scripts/gen_token.go`, no paste).
- [ ] **AC3 — `packing_lists.http` reverted to chained setup.** The
      self-contained per-section setup/cleanup blocks (PACK-012-onward,
      marked "👉 NEW IN THIS COMMIT") are replaced with sections chaining
      off one shared Setup block at the top of the file, matching the
      style already used in the other 4 files. Same test-case coverage as
      today — this is a restructuring, not a coverage change.
- [ ] **AC4 — 401 checks trimmed.** Each file retains only 1-2
      representative "no token → 401" checks (one read endpoint, one
      write endpoint) instead of one after every route.
- [ ] **AC5 — Cleanup consolidated.** Each file ends with a single cleanup
      block that removes everything the file created, in reverse
      dependency order, replacing any per-section duplicate cleanup.
- [ ] **AC6 — `requests/README.md` added.** Documents: how to get a token
      (`go run scripts/gen_token.go`), how host/token resolve via
      `.env`/`$dotenv`, the "run top-to-bottom" usage model, and the
      cleanup convention.
- [ ] **AC7 — No live tokens committed.** Confirm neither
      `templates.http` nor `template_items.http` (nor any other file)
      retains a pasted real token value after the rewrite.

## Non-goals

- `requests/auth.http` — not created this ticket (browser-driven OAuth
  flow needs its own design; already flagged separately).
- Smoke-test vs. full-regression file/section split — not built. The
  per-commit fast-iteration use case that motivated it is deprioritized
  for now.
- Automated Go test for `gen_token.go` — explicitly exempted as a
  build-ignored manual dev-tooling script.
- Temporary/uncommitted scratch `.http` files for isolated testing of a
  future feature — raised as a possible future pattern, not scoped or
  built here.
- Rotating/purging the expired dev JWTs already sitting in git history in
  `templates.http`/`template_items.http` — they're expired, dev-only,
  signed against a local secret for a placeholder user; no live-credential
  risk, no action taken beyond structurally preventing recurrence (AC1-2).
- Redesigning the visual conventions (✅/❌/🔒 emoji, `═══` section
  banners, one-file-per-resource split) — carried over as-is; only the
  chaining/cleanup/token/host mechanics change.

## Expected test files

No automated Go test files — this ticket only touches `.http` files and
the `scripts/gen_token.go` dev tool (see non-goals above).

Manual verification *is* the deliverable, checked per commit boundary:

- **After commit 1 (tooling):** run `go run scripts/gen_token.go` against
  the local server/Neon dev DB, confirm `.env` gains/updates a `DEV_TOKEN`
  line without disturbing other vars.
- **After commit 2 (all `.http` files + README):** with the server running
  locally, run each of the 5 files top-to-bottom in VS Code REST Client
  and confirm:
  - `{{$dotenv DEV_TOKEN}}` / `{{$dotenv PORT}}` resolve with no manual
    paste needed anywhere.
  - Every request in every file returns its documented expected status.
  - Each file's cleanup block leaves the DB in the same state it started
    in (no leftover rows) — re-running a file twice in a row should behave
    the same both times.
