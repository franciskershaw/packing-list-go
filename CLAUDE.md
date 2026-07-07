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

## Docs

- `docs/specs/master-spec.md` — living spec + ticket backlog
- `docs/handoffs/PACK-NNN.md` — one per ticket
- `LESSONS.md` — retro log, reviewed each kickoff/grill-me
