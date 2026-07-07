# PACK-005 — Token refresh and logout

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from
> `internal/handler/auth_handler.go` (`RefreshToken`, `Logout`) and
> [[project_architecture]] memory. This is documentation of what was
> built, not a spec that preceded it.

## Context

Let a signed-in user get a new access token without re-authenticating with
Google every 15 minutes, and let them log out.

## Acceptance criteria

- [x] `POST /auth/refresh` reads the `refreshToken` cookie, validates it,
      looks up the user by the token's `sub`, and returns a new access
      token. 401 if the cookie is missing, invalid, or the user no longer
      exists.
- [x] `POST /auth/logout` clears the `refreshToken` cookie (max-age -1) and
      returns 200.

## Known trade-off (accepted, not a bug)

Logout only clears the refresh cookie. The access token (15-minute TTL)
remains valid until it naturally expires — there is no server-side token
blacklist. This was a deliberate scope decision, not an oversight; revisit
only if a real requirement for immediate token revocation shows up (e.g.
"kill all sessions" after a security incident).

## Non-goals / files not touched by this ticket

- No token blacklist / revocation list.
- No "log out of all devices" — only the current refresh cookie is cleared.

## Tests

`internal/handler/auth_handler_test.go`.
