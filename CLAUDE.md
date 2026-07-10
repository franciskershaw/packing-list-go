# Packing List API

Follows the global development process — see `~/.claude/CLAUDE.md`.

## Naming

Working title only. Avoid introducing a product name into code, docs, or
endpoint naming until one is chosen — use generic terms ("packing list
API", "this service") instead.

## Stack

- Go, Gin, PostgreSQL (Neon)
- Auth: Google OAuth2/OIDC login + custom JWT access (15 min) / refresh
  (7 day, httponly cookie) tokens. No server-side session store or
  blacklist.
- Migrations: plain up/down SQL files in `db/migrations/`.

## Architecture

- **Repository interfaces are defined in the `handler` package**
  (consumer-defined), not in `repository`. Each `Postgres*Repository` under
  `internal/repository` implements the interface(s) its handlers need.
- **Handlers are structs** with injected dependencies (repo, and for auth,
  the OAuth manager + config) — not standalone functions.
- **Ownership model**: `user_id IS NULL` = system-level row, visible to
  everyone but not user-editable. Non-null `user_id` = owned by that user,
  scoped on every read/write. Helpers in `internal/handler/ownership.go`.
- **Tests**: `package handler_test` (black-box), `httptest.NewRecorder()` +
  `gin.New()`. Shared helper `internal/testutil/auth.go` (`AuthHeader`)
  generates a Bearer token for tests without going through the Google flow.
- **Implementation order**: per the global layer-by-layer implementation
  rule, implement the repository layer (and its integration tests) fully,
  commit, and pause for review before starting on handler code for the
  same ticket — handler tests run entirely against a mocked repository, so
  the two layers have no real coupling to build in the same pass.

## Overrides of the global default process

- **Testing library**: the global default is stdlib-only tests. This
  project overrides that — handler tests use `testify/mock` for repository
  mocks (see `internal/handler/category_handler_test.go`,
  `item_handler_test.go`, `auth_handler_test.go`). Keep using
  `testify/mock` for new handler tests rather than introducing a second,
  hand-rolled mocking style. Confirmed as an explicit override
  (2026-07-07), not an oversight.

## Testing

- **Handler tests** need no database. Run with: `go test ./internal/handler/...`
  New handler test files should use the shared `doRequest` helper in
  `internal/handler/handler_test_helpers_test.go` to send requests
  (`req := httptest.NewRequest(...)` + headers + `NewRecorder` +
  `ServeHTTP`, collapsed to one call) rather than repeating that block per
  test — existing files (`category_handler_test.go`,
  `item_handler_test.go`, etc.) predate this helper and are a tech-debt
  retrofit candidate, not something to rewrite incidentally.
- **Repository tests** are integration tests that hit the real Neon dev
  database. Run with: `DATABASE_URL=$DATABASE_URL go test ./internal/repository/...`
  Never spin up Docker or a local database instance for tests on this project.
- **TDD in Go**: write the test file first, then create a minimal stub
  (empty handler struct + method bodies) so the test file compiles. Stub
  bodies must produce an assertable failure, not a panic: repository stubs
  return `nil, errors.New("not implemented")`; handler stubs write a real
  (if wrong) HTTP response, e.g. `http.StatusNotImplemented`. A literal
  `panic()` gets recovered and then re-raised by Go's `testing.tRunner`,
  aborting the entire test binary on the first stub hit — so only one
  test's failure is visible per run instead of the whole suite's. Run
  tests to confirm every test fails at runtime for the right reason
  before implementing.

## Docs

- `docs/specs/master-spec.md` — living spec + ticket backlog
- `docs/handoffs/PACK-NNN.md` — one per ticket
- `LESSONS.md` — retro log, reviewed each kickoff/grill-me

## Off-scope findings during feature work

The global rule is to flag, not silently fix, anything outside a ticket's
stated scope. In this project, that flag has a home: suggest adding it to
**Epic 6 ("Codebase Health & Hardening")** in `docs/specs/master-spec.md`
rather than just mentioning it and letting it evaporate once the
conversation moves on. That epic exists specifically as the durable
landing spot for security/architecture/consistency findings that surface
mid-feature — currently drafted at `docs/handoffs/PACK-014.md`, not yet
scoped via `grill-me`. This applies whether the finding comes from Claude's
own review or a human/other-agent review shared in chat.
