# PACK-019 — Handler test `doRequest` retrofit

## Context

Part of Epic 6 (Codebase Health & Hardening). Source finding: item 10 in
`docs/handoffs/epic-6-findings.md` — `httptest.NewRequest(...)` +
`Content-Type`/`Authorization` headers + `httptest.NewRecorder()` +
`r.ServeHTTP(...)` repeats verbatim across handler test files instead of
using the shared `doRequest` helper
(`internal/handler/handler_test_helpers_test.go`).

Key decisions from the interview:

- **The archived finding's premise was wrong about scope.** It claimed
  `doRequest` is "already used by every packing-list test file." Checked
  directly: only `packing_list_item_handler_test.go` (44 uses) and
  `packing_list_pack_handler_test.go` (12 uses) actually use it.
  `packing_list_handler_test.go` — the core CRUD file — has 37 raw
  `httptest.NewRequest` blocks, same problem as the 4 files the ticket
  originally named. Decided to include it: **5 files, 167 call sites
  total**, not the 4 files / 130 sites originally scoped. Same reasoning
  as PACK-018's `bindJSON` discovery — the pattern and fix are identical,
  leaving it out just defers rediscovery to a later ticket.
  - `category_handler_test.go` — 25
  - `item_handler_test.go` — 37
  - `template_handler_test.go` — 30
  - `template_item_handler_test.go` — 38
  - `packing_list_handler_test.go` — 37
- **Verified every site is a safe mechanical substitution** before
  committing to the approach: router variable is consistently `r`,
  recorder variable is consistently `w`, and no site sets any header
  beyond `Content-Type`/`Authorization` (both already parameters of
  `doRequest`). One wrinkle found: `category_handler_test.go`'s
  `TestCreate_MalformedJSON` (added in PACK-018) passes
  `strings.NewReader(...)` as the body, which doesn't satisfy
  `doRequest`'s current `body *bytes.Buffer` parameter type.
- **Fix for the wrinkle**: widen `doRequest`'s signature to
  `body io.Reader` in `handler_test_helpers_test.go`. Fully
  backward-compatible — `*bytes.Buffer` already satisfies `io.Reader`, so
  every existing call site (including the 2 files already using it)
  keeps working unchanged, and `TestCreate_MalformedJSON` can now also
  use `doRequest` instead of being left as the one raw exception in an
  otherwise fully-retrofitted file.
- **`auth_handler_test.go`'s 11 raw call sites are explicitly out of
  scope** — never named in the original finding, and genuinely
  incompatible with `doRequest`'s signature as-is: several tests need
  `req.AddCookie(...)` before sending (for the refresh-token cookie),
  which `doRequest` has no parameter for. Widening `doRequest` further to
  support cookies would be new scope beyond "retrofit callers of the
  existing helper."
- **Purely mechanical — zero behavior change.** Every retrofitted test
  keeps its exact assertions; only the request-construction plumbing
  changes. Verification per file/AC is a before/after comparison of that
  file's own test run (same test names, same pass/fail results, same
  count) rather than new test cases — there's no new behavior to test.
- **No manual verification / `.http` file** — consistent with every
  other Epic 6 ticket so far; this changes zero API-visible behavior.

## Acceptance criteria

- [x] **AC1 — `category_handler_test.go` retrofitted (25 sites).**
  Includes widening `doRequest`'s `body` parameter to `io.Reader` in
  `handler_test_helpers_test.go` (needed for this file's
  `TestCreate_MalformedJSON`). All of this file's tests pass, identical
  names/count to before.
- [x] **AC2 — `item_handler_test.go` retrofitted (37 sites).** All tests
  pass, identical names/count to before.
- [x] **AC3 — `template_handler_test.go` retrofitted (30 sites).** All
  tests pass, identical names/count to before.
- [x] **AC4 — `template_item_handler_test.go` retrofitted (38 sites).**
  All tests pass, identical names/count to before.
- [x] **AC5 — `packing_list_handler_test.go` retrofitted (37 sites).**
  All tests pass, identical names/count to before.

## Non-goals

- `auth_handler_test.go`'s 11 raw call sites — incompatible with
  `doRequest` as-is (cookie handling); not this ticket's job to extend
  `doRequest` further to support it.
- No change to any test's assertions, mocks, or fixtures — request
  construction only.
- No further `doRequest` signature changes beyond the `io.Reader`
  widening.
- `requests/*.http` structural rethink — PACK-020.
- No manual verification / no `.http` file (see Context).

## Expected test files

No new test files or test cases — this ticket modifies existing test
files' internal plumbing only:

- `internal/handler/handler_test_helpers_test.go` (**modified**):
  `doRequest`'s `body` parameter widened from `*bytes.Buffer` to
  `io.Reader`.
- `internal/handler/category_handler_test.go`,
  `item_handler_test.go`, `template_handler_test.go`,
  `template_item_handler_test.go`, `packing_list_handler_test.go`
  (**modified**): every raw `httptest.NewRequest(...)` +
  headers + `httptest.NewRecorder()` + `r.ServeHTTP(...)` block replaced
  with a single `doRequest(t, r, method, path, body, authHeader)` call.
- Verification per AC: `go test ./internal/handler/... -v` before and
  after each file's retrofit, confirming identical test names and 0
  failures — not a new test, a regression check against the existing
  suite.

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
