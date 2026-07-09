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
- **Repository tests** are integration tests that hit the real Neon dev
  database. Run with: `DATABASE_URL=$DATABASE_URL go test ./internal/repository/...`
  Never spin up Docker or a local database instance for tests on this project.
- **TDD in Go**: write the test file first, then create a minimal stub
  (empty handler struct + method bodies that `panic("not implemented")`)
  so the test file compiles. Run tests to confirm runtime failures before
  implementing. Do not write implementation code until you have seen the
  tests fail at runtime.

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
