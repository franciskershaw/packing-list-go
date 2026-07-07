# PACK-003 — Auth middleware

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from
> `internal/middleware/auth.go` and the git log (`e02f3a9`). This is
> documentation of what was built, not a spec that preceded it.

## Context

Gate authenticated routes behind a Bearer access token, and make the
authenticated user's ID/email available to downstream handlers without each
one re-parsing the token.

## Acceptance criteria

- [x] Requests without an `Authorization` header get 401.
- [x] Requests with a malformed header (not exactly `Bearer <token>`) get
      401.
- [x] Requests with an invalid/expired access token get 401.
- [x] On success, `userId` and `email` are set on the Gin context for
      handlers to read (see `userIDFromCtx` in
      `internal/handler/context.go`).

## Non-goals / files not touched by this ticket

- Refresh-token handling lives in the auth handler (PACK-005), not here —
  this middleware only ever looks at the access token.

## Tests

No dedicated middleware test file exists; coverage comes indirectly through
handler tests that exercise authenticated routes (e.g.
`internal/handler/category_handler_test.go`). If middleware logic grows
more branches, it should get its own `internal/middleware/auth_test.go`.
