# PACK-023 — OAuth state store fix

## Context

Source: `docs/handoffs/audit-2026-07-11-findings.md` item 5. `GenerateState`
(`internal/auth/google.go`) adds an entry to an unbounded
`map[string]time.Time` on every hit to the unauthenticated
`/auth/google/login`; entries are only ever evicted when that exact state
string is later re-validated by `ValidateState` (success or expiry check).
Abandoned flows (closed browser, bot traffic hitting the login endpoint)
leak forever — the map only shrinks via `ValidateState`'s own `delete`
calls, and no sweep goroutine exists anywhere in the codebase. It also
doesn't survive restarts or horizontal scaling.

Also referenced in the most recent `LESSONS.md` entry (PACK-022,
2026-08-01): `ulule/limiter/v3` was chosen over hand-rolling specifically
"to avoid PACK-023's exact unbounded-map bug class" — this ticket is that
bug class's own fix.

**Design gate**: follows precedent, no ADR needed. This reuses two
existing patterns rather than introducing a new one:
- `internal/auth/jwt.go`'s `signToken` helper (`golang-jwt/jwt/v5`, HS256)
  and `ValidateAccessToken`/`ValidateRefreshToken`'s shape (`tokenString
  string, secret string) (*Claims, error)` — pure token-in,
  claims/error-out, no HTTP concerns.
- `internal/handler/auth_handler.go`'s `setRefreshCookie` (`HttpOnly`,
  `SameSite=Lax`, `Secure` gated on `cfg.Environment == "production"`,
  `Path="/"`).
No existing ADR touches OAuth state or JWT secret handling, so nothing to
supersede.

Key decisions from interview:

- **Fix direction**: go fully stateless with a signed JWT cookie, not a
  bounded map + sweep. Removes `stateStore`/`stateStoreMutex` from
  `GoogleOAuthManager` entirely — structurally eliminates the "unbounded"
  bug class rather than bounding it, and also fixes the
  restart/horizontal-scaling gap the sweep option wouldn't have.
- **Shape**: `GenerateState` mints a JWT (`jwt.RegisteredClaims` with only
  `ExpiresAt` set — no other claims needed) and returns that *same JWT
  string* for two uses: (a) the `state` query param passed to Google via
  `GetAuthURL` (unchanged signature), and (b) set as an `HttpOnly` cookie
  by `LoginWithGoogle`. `GoogleCallback` then does a **double-submit
  check**: compare the query-param `state` to the cookie value (plain
  string equality — this is what actually defeats CSRF, proving the
  callback came from the same browser that started the flow), then call
  `ValidateState` to verify the JWT's signature and expiry. Cookie is
  cleared (`Max-Age -1`) after the check regardless of outcome.
- **Signing secret**: new dedicated `config.Config.JWTSecretOAuthState`
  (env `JWT_SECRET_OAUTH_STATE`), required — fails `Load()` hard if unset,
  same pattern as `JWTSecretAccess`/`JWTSecretRefresh`
  (`config/config.go:40-52`). Not a reuse of an existing secret — keeps a
  third token class on a third secret, consistent with this codebase's
  intent (see below) that JWT secrets stay distinct per token type.
  **Off-scope finding**: PACK-028's backlog item "JWT-secrets-distinct
  startup assertion" doesn't exist in code yet (confirmed via grep — only
  the two empty-string checks exist today, nothing compares secrets
  against each other). Flagging that when PACK-028 is picked up, its
  assertion should cover all three secrets pairwise, not just
  access/refresh — not built now, this ticket only adds the third secret
  and its own empty-string check.
- **TTL**: unchanged from today's behavior — 10 minutes, but now a
  package-level const in `internal/auth/google.go` (e.g. `oauthStateTTL =
  10 * time.Minute`) baked into the JWT's `exp` claim, rather than a
  per-instance struct field. Matches how `refreshTokenTTL`/
  `refreshGraceWindow` are already handler-package consts, not config
  fields — no precedent in this codebase for exposing auth timing values
  via env vars.
- **Error handling**: `GenerateState`'s signature changes from `() string`
  to `() (string, error)`, matching `GenerateAccessToken`/
  `GenerateRefreshToken`'s existing convention (they already return
  `error` on a `signToken` failure). `LoginWithGoogle` responds `500
  gin.H{"error": "internal server error"}` on that error instead of the
  current panic-on-`crypto/rand`-failure behavior. (`crypto/rand` itself
  is no longer called at all — the JWT's own signature is the only
  randomness/entropy source needed; no separate random state string is
  generated.)
- **Layer split**: the cookie-vs-query-param equality check lives in the
  **handler** (`GoogleCallback`), not inside `ValidateState`. Keeps
  `ValidateState(state string) bool`'s signature and shape unchanged
  (still verifies one token string, still returns a bool) — mirrors
  `ValidateAccessToken`/`ValidateRefreshToken`'s pure-crypto,
  no-HTTP-concerns shape. The `OAuthManager` interface
  (`internal/handler/auth_handler.go:44-50`) only changes at
  `GenerateState`'s signature; `ValidateState`, `GetAuthURL`,
  `ExchangeCodeForToken`, `VerifyIDToken` are untouched.
- **Replay tradeoff, explicitly accepted**: dropping server-side storage
  means strict one-time-use enforcement is gone — a captured
  cookie+query-param pair could theoretically be replayed before the
  cookie is cleared or the JWT expires. Accepted as fine: the `state`
  param's actual job is CSRF protection (same-browser proof), not
  replay-proofing, and the Google-issued authorization `code` is itself
  single-use and short-lived server-side (enforced by Google) — a
  replayed `state` alone can't produce a usable `code`. This is the
  standard double-submit-cookie pattern most stateless OAuth
  implementations use. Clearing the cookie after use is defense-in-depth,
  not a correctness requirement.
- **Cookie name**: `oauthState`, following the existing `refreshToken`
  cookie's camelCase naming.

## Acceptance criteria

- [ ] AC1 — `GoogleOAuthManager` no longer holds `stateStore`/
      `stateStoreMutex` (removed from the struct and
      `newGoogleOAuthManager`). `GenerateState() (string, error)` returns
      a JWT signed with `cfg.JWTSecretOAuthState`, `exp` set to
      `time.Now().Add(oauthStateTTL)` (10 min, package const), no other
      claims.
- [ ] AC2 — `LoginWithGoogle` sets the returned JWT as an `HttpOnly`
      cookie named `oauthState` (`SameSite=Lax`, `Secure` gated on
      `cfg.Environment == "production"`, `Path="/"`, matching
      `setRefreshCookie`'s attributes) in addition to passing it as the
      `state` query param via `GetAuthURL` (unchanged). Responds `500
      gin.H{"error": "internal server error"}` if `GenerateState` errors,
      instead of panicking.
- [ ] AC3 — `GoogleCallback` reads the `oauthState` cookie and compares it
      by plain string equality to the query-param `state` *before*
      calling `ValidateState`; a missing cookie or a mismatch is a `401
      gin.H{"error": "invalid state parameter"}` — same response shape as
      today's invalid-state path. The cookie is cleared (`Max-Age -1`)
      after the check, on both the success and failure paths.
- [ ] AC4 — `ValidateState(state string) bool` verifies the JWT's
      signature (`cfg.JWTSecretOAuthState`) and expiry instead of a map
      lookup; interface signature and return type unchanged.
- [ ] AC5 — `config.Config` gains a required `JWTSecretOAuthState` field
      (env `JWT_SECRET_OAUTH_STATE`), following the exact pattern of
      `JWTSecretAccess`/`JWTSecretRefresh` (`config/config.go:40-52`) —
      `Load()` fails hard with `fmt.Errorf("JWT_SECRET_OAUTH_STATE not
      set")` if unset. Added to `.env.example` (after
      `JWT_SECRET_REFRESH=`).

## Non-goals

- Bounded map + sweep goroutine — explicitly rejected direction (see
  Context). No periodic sweep/ticker is added anywhere by this ticket.
- PACK-028's "JWT-secrets-distinct startup assertion" — not built now;
  flagged as an off-scope finding for that ticket to cover all three
  secrets when picked up.
- Any change to `ExchangeCodeForToken`, `VerifyIDToken`, `GetAuthURL`'s
  signature, or the access/refresh token JWT logic in `internal/auth/jwt.go`.
- `requests/auth.http` — not touched. `requests/README.md`'s "what's
  deliberately not covered" section already documents that Google
  login/callback needs a real browser round-trip and can't be driven by a
  plain `.http` request; that reasoning is unchanged by this ticket.
- Moving `oauthStateTTL` into `config.Config` — stays a package const, see
  Context.
- Any change to `/auth/refresh` or `/auth/logout` behavior (PACK-027's
  refresh-token rotation is untouched).

## Expected test files

- `internal/auth/google_test.go` (rewrite the four existing state-related
  tests — current versions assume the map-based design):
  - `TestGenerateState` — asserts a non-empty, well-formed JWT string is
    returned with a nil error; parses it back and checks the `exp` claim
    is ~10 minutes out.
  - `TestValidateState_ValidToken` — a freshly generated state validates
    true.
  - `TestValidateState_InvalidSignature` — a token signed with a
    different secret fails.
  - `TestValidateState_Expired` — construct an already-expired JWT
    directly via the `jwt` library (no more sleeping on an overridden
    `stateExpiryTime` field — that field no longer exists) and assert
    `ValidateState` returns false.
  - `TestGetAuthURL` — unchanged, still asserts `client_id`/
    `redirect_uri`/`state` in the built URL.
- `internal/handler/auth_handler_test.go`:
  - Update `MockOAuthManager.GenerateState` to match the new `(string,
    error)` signature; update every `.On("GenerateState")` stub across
    the ~26 existing call sites in this file that exercise
    `LoginWithGoogle`/`GoogleCallback`.
  - `TestLoginWithGoogle_SetsOAuthStateCookie` — asserts the response sets
    an `oauthState` cookie with the expected attributes.
  - `TestLoginWithGoogle_GenerateStateError_Returns500` — mocked
    `GenerateState` error path.
  - `TestGoogleCallback_MissingCookie_Returns401`
  - `TestGoogleCallback_CookieMismatch_Returns401`
  - `TestGoogleCallback_ClearsCookieAfterCheck` — asserts the cookie is
    cleared on both a success and a failure path.
  - Existing `TestGoogleCallback_InvalidState_Returns401` (or equivalent)
    updated to seed a matching cookie so it's still testing
    `ValidateState`'s own failure path, not the new cookie check.
- `config/config_test.go` (add to the existing file, mirroring
  `TestLoad_...` structure):
  - `TestLoad_MissingJWTSecretOAuthState_ReturnsError`
- Manual verification: a real browser round-trip against the dev Google
  OAuth app — hit `/auth/google/login`, confirm the `oauthState` cookie is
  set with the expected attributes via devtools, complete the Google
  consent screen, confirm the callback succeeds and the cookie is cleared
  afterward. Not a `requests/auth.http` addition (see Non-goals).

## Close-out

Not started.
