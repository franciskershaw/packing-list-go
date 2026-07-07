# PACK-004 — Google OAuth login flow

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from
> `internal/auth/google.go`, `internal/auth/google_test.go`,
> `internal/handler/auth_handler.go`, and the git log (`c049a9c`
> "Speculative google auth flow", `af3a072`, `c967bae`). This is
> documentation of what was built, not a spec that preceded it.

## Context

Let a user sign in with their Google account instead of a
username/password. On success, create or update their local user record and
issue this API's own JWTs so the rest of the service doesn't need to know
anything about Google.

## Acceptance criteria

- [x] `GET /auth/google/login` generates a CSRF state token (stored
      in-memory with a 10-minute expiry) and redirects to Google's OAuth
      consent screen.
- [x] `GET /auth/google/callback`:
  - [x] 400 if `code` or `state` query params are missing.
  - [x] 401 if `state` is missing from the store or expired (invalid/reused
        CSRF token).
  - [x] Exchanges the code for tokens, verifies the ID token via the OIDC
        provider, extracts email/Google ID/name/avatar from claims.
  - [x] Gets-or-creates the local user (`UserRepository.GetOrCreateUser`),
        updating `last_login_at` on an existing user.
  - [x] Issues an access token (returned in the JSON body) and a refresh
        token (set as an httponly cookie, 7-day expiry).
- [x] `GoogleOAuthManager` is initialized once at startup in `main.go`
      (it makes a network call to Google's OIDC discovery endpoint) and
      injected into `AuthHandler` behind an `OAuthManager` interface so it
      can be mocked in tests.

## Non-goals / files not touched by this ticket

- Token refresh and logout are PACK-005.
- No other OAuth providers.

## Tests

`internal/auth/google_test.go`, `internal/handler/auth_handler_test.go`.
