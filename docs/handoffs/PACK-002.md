# PACK-002 — JWT helpers

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from `internal/auth/jwt.go`,
> `internal/auth/jwt_test.go`, and the git log (`1ac9ede`, `022e60d`). This
> is documentation of what was built, not a spec that preceded it.

## Context

Generate and validate JWT access and refresh tokens so the rest of the auth
flow (PACK-004/005) and the auth middleware (PACK-003) have something to
build on.

## Acceptance criteria

- [x] `GenerateAccessToken(email, userId)` returns a signed HS256 JWT
      containing `email`, `userId`, and a 15-minute expiry, signed with
      `JWT_SECRET_ACCESS`.
- [x] `GenerateRefreshToken(userId)` returns a signed HS256 JWT with the
      user ID as `sub` and a 7-day expiry, signed with `JWT_SECRET_REFRESH`.
- [x] `ValidateAccessToken` / `ValidateRefreshToken` parse and verify
      signature + expiry, rejecting tokens signed with an unexpected
      algorithm.
- [x] Missing secret env vars produce an error rather than signing with an
      empty key.

## Non-goals / files not touched by this ticket

- No token storage/blacklist — out of scope here and for the whole project
  (see the JWT logout trade-off noted under PACK-005).
- No middleware — that's PACK-003.

## Tests

`internal/auth/jwt_test.go` — moved here from elsewhere in the tree per
commit `5a19c64` ("Move JWT unit tests to auth/jwt_test.go").
