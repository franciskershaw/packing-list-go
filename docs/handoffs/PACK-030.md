# PACK-030 — Session-restore / user-profile endpoint

Add `GET /me`, returning the authenticated user's profile, and fix
`GoogleCallback`'s response to actually include `avatarUrl`.

## Context

`POST /auth/refresh` returns only a new `accessToken`
(`internal/handler/auth_handler.go`), so a persistent-sign-in flow has no
way to restore name/avatar without a fresh OAuth login. Found 2026-07-11
during frontend prototyping (not the Epic 6/7 audit). Also fixes a related
gap found during verification: `GoogleCallback`'s response never returns
`avatarUrl` either, despite it being captured and stored server-side
(`GetOrCreateUser`'s `avatarURL` param, `models.User.AvatarURL`) — so
`GET /me` would otherwise be the *only* way to ever expose it, not just
the refresh-time fix.

**Design gate**: follows precedent, no new pattern, no ADR needed (no
`docs/adr/` yet in this project). Key decisions from the interview:

- **Scope**: both the new endpoint and the `GoogleCallback` `avatarUrl`
  fix stay in this one ticket — same root cause (avatarUrl never
  surfaced to any client), same handler file, small diff either way.
- **Route**: `GET /me`, in the JWT-access-token `authed` group in
  `main.go` (alongside `/categories`, `/items`, `/lists`), not the
  refresh-cookie path `RefreshToken` uses. `userID` comes from
  `userIDFromCtx`, same as every other authed-group handler.
- **Response shape**: flat top-level `{id, email, name, avatarUrl}` — no
  `"user"` wrapper. `GET /me`'s entire payload *is* the user, unlike
  `GoogleCallback` which nests `"user"` alongside a sibling `accessToken`
  field.
- **User lookup / not-found handling**: mirrors `RefreshToken`
  (`auth_handler.go:114-145`) exactly — `userRepo.GetUserByID`, `nil, nil`
  → `401 {"error": "user not found"}` (access token outlives the user
  row being deleted, however unlikely in the 15-min window), repo error →
  `500`. Cited precedent: `TestRefreshToken_UserNotFound` /
  `TestRefreshToken_UserLookupError` (`auth_handler_test.go:286, 316`).
- **File placement**: `Me` method added to `AuthHandler` in
  `auth_handler.go`, alongside `LoginWithGoogle`/`GoogleCallback`/
  `RefreshToken`/`Logout` — no ambiguity here, it's the same handler
  struct and file every other auth endpoint already lives in.
- **Test style**: new tests in `auth_handler_test.go` use the shared
  `doRequest` helper (`handler_test_helpers_test.go`), per project
  CLAUDE.md's guidance for new handler test code — existing raw
  `httptest.NewRequest` tests in this file predate the helper and are a
  tech-debt retrofit candidate, not a pattern to keep extending.
- **Manual verification**: creates `requests/auth.http` (previously
  nonexistent — see `requests/README.md`'s "what's deliberately not
  covered" section) covering just `GET /me`. Login/callback/refresh/
  logout remain out of scope for `.http` coverage, per that same README
  note, which gets updated to reflect that `GET /me` is now covered.

## Acceptance criteria

- [ ] `GET /me` returns `200` with `{id, email, name, avatarUrl}` for a
      valid access token.
- [ ] `401 {"error": "user not found"}` if the token's subject no longer
      resolves to a user row (`GetUserByID` returns `nil, nil`).
- [ ] `500` if `GetUserByID` returns an error.
- [ ] `401` for a missing/invalid access token (via existing
      `AuthMiddleware`, same as every other authed route).
- [ ] `GoogleCallback`'s JSON response's nested `"user"` object gains
      `avatarUrl` (alongside existing `id`/`email`/`name`).

## Non-goals

- No change to `POST /auth/refresh`'s response shape — it still returns
  only `accessToken`. Only `GoogleCallback` and the new `GET /me` return
  full profile fields.
- No `.http` coverage added for login/callback/refresh/logout — still out
  of scope per `requests/README.md` (browser round-trip / separate future
  work, unchanged by this ticket).
- No new repository method — `GetUserByID` already exists and is reused
  as-is.
- Not touching PACK-021 (server lifecycle) or PACK-029 (done) — no code
  overlap.

## Expected test files

- `internal/handler/auth_handler_test.go` — add:
  - `TestMe_Success` — valid access token, mocked `GetUserByID` returns
    `testUser()`, asserts `200` and all four flat fields.
  - `TestMe_UserNotFound` — mocked `GetUserByID` returns `nil, nil`,
    asserts `401` and the exact `"user not found"` error message, mirroring
    `TestRefreshToken_UserNotFound` (line 286).
  - `TestMe_UserLookupError` — mocked `GetUserByID` returns `nil, err`,
    asserts `500`, mirroring `TestRefreshToken_UserLookupError` (line 316).
  - `TestMe_Unauthorized` — no/invalid auth header, asserts `401`.
  - `TestGoogleCallback_HappyPath` (existing, line 118) — extend the
    existing body assertions to also check `avatarUrl` on the nested
    `user` object, rather than adding a near-duplicate test.
  - Test router: `newTestRouter` (line 87) needs a `GET /me` route
    wrapped in `middleware.AuthMiddleware(testutil.TestJWTSecretAccess)`,
    matching how `newPackingListTestRouter` wires its `authed` group
    (`packing_list_handler_test.go:127-144`).
- `requests/auth.http` (new file) — `GET /me` happy path plus a `401`
  check (missing/invalid token), following the shared conventions in
  `requests/README.md` (`{{$dotenv DEV_TOKEN}}`, `{{$dotenv PORT}}`).
  `requests/README.md`'s "what's deliberately not covered" section gets
  a one-line update noting `GET /me` is now covered, scoping the
  "auth.http doesn't exist" statement down to login/callback/refresh/
  logout only.

## Close-out

Completed 2026-07-14. Retro entry in LESSONS.md. Follow-up finding filed
as PACK-031 (`avatar_url` NOT NULL constraint + repo-test regression
guard) in `docs/specs/master-spec.md`.
