# PACK-014 — Security hardening: refresh-token cookie & OAuth CSRF token

## Context

Part of Epic 6 (Codebase Health & Hardening). Source findings: items 1–2
in `docs/handoffs/epic-6-findings.md` (moved there 2026-07-10 from this
file's old location — that file remains the shared reference archive for
PACK-015–020; this doc is PACK-014's own real, implementation-ready
handoff, produced via `grill-me` on 2026-07-10).

Two independent, low-effort security findings from a 2026-07-09 codebase
review, bundled into one ticket because both are small and same-risk-class:

1. The refresh-token cookie (`internal/handler/auth_handler.go`) is set
   with `Secure=false` unconditionally and no explicit `SameSite`,
   relying on browser defaults.
2. The OAuth CSRF state token (`internal/auth/google.go`'s
   `GenerateState`) is built from `math/rand`, not `crypto/rand`.

Key decisions from the interview:

- **No `Environment`/`APP_ENV` concept exists yet in `config.Config`** —
  only ad hoc in `scripts/gen_token.go` (a `//go:build ignore` dev
  script). This ticket adds a real `Environment` field to `config.Config`
  (default `"development"` via `getEnv("APP_ENV", "development")`,
  matching the existing `PORT` pattern) and threads it into `AuthHandler`
  (which already receives `*config.Config` but doesn't use it). `Secure`
  is `true` only when `cfg.Environment == "production"`. This is a new
  field addition, not the "thread `DATABASE_URL`/JWT secrets through
  `db.go`/`jwt.go`" work — that's PACK-015's separate scope.
- **`SameSite=Lax`**, set explicitly via `c.SetSameSite(...)` before
  `c.SetCookie(...)`. No CORS/cross-origin frontend exists yet (API-only
  per the master spec's non-goals), so `Lax` is the safe default — but
  flag this in a code comment as needing revisiting once a real frontend
  consumer exists, since that may require `None`+cross-origin handling.
- **CSRF fix keeps the existing charset-based algorithm** — swap
  `rand.Intn(len(charset))` for `crypto/rand.Int(rand.Reader,
  big.NewInt(int64(len(charset))))` per character, rather than switching
  to raw-byte + base64/hex encoding. Preserves the exact 32-char
  alphanumeric output format so `TestGenerateState`/`TestValidateState`
  in `internal/auth/google_test.go` need no changes.
- **No automated test can distinguish `crypto/rand` from `math/rand`** at
  runtime — this AC is verified by code review of the import/call site,
  not a new assertion. The existing `TestGenerateState` (length==32,
  non-empty) and `TestValidateState` (one-time-use) continue to serve as
  regression coverage for the unchanged output contract. Recorded
  explicitly per the standing rule that a listed "test" which doesn't
  cover new behavior should say so rather than look like ordinary
  coverage.
- **Shared `setRefreshCookie(c *gin.Context, value string, maxAge int)`
  helper** on `AuthHandler`, wrapping the `SetSameSite`+`SetCookie` logic,
  since `GoogleCallback` (issues) and `Logout` (clears) need identical
  `Secure`/`SameSite` behavior and only differ in value/maxAge.
- **All 8 existing `NewAuthHandler(userRepo, oauthMgr, nil)` call sites**
  in `auth_handler_test.go` currently pass `nil` for `cfg`. Once
  `GoogleCallback`/`Logout` read `h.cfg.Environment`, every one of those
  tests would nil-pointer-panic, not just the two that assert on cookies.
  All 8 get updated to a new `testConfig(environment string)
  *config.Config` helper.
- **No `requests/*.http` file for this ticket.** Explicit decision: the
  user is unhappy with the current `.http` file conventions and PACK-020
  (already scoped in the master spec) will rethink the whole suite
  structurally. Starting a `requests/auth.http` now risks being thrown
  away/restructured by that work. This is a deliberate deviation from the
  global process's per-feature manual-verification step, made explicitly
  in this interview rather than silently skipped — noted also in
  `docs/handoffs/epic-6-findings.md` item 12.

## Acceptance criteria

- [x] **AC1 — Refresh-token cookie `Secure`/`SameSite`.**
  - `config.Config` has a new `Environment string` field, populated by
    `Load()` via `getEnv("APP_ENV", "development")`.
  - `AuthHandler` gets a private `setRefreshCookie(c *gin.Context, value
    string, maxAge int)` helper that calls `c.SetSameSite(http.SameSiteLaxMode)`
    then `c.SetCookie("refreshToken", value, maxAge, "/", "",
    h.cfg.Environment == "production", true)`.
  - `GoogleCallback` (currently `auth_handler.go:93`) and `Logout`
    (currently `auth_handler.go:135`) both call this helper instead of
    calling `c.SetCookie` directly.
  - With `Environment == "development"` (or unset), the cookie's `Secure`
    attribute is `false`. With `Environment == "production"`, it's `true`.
    `SameSite` is `Lax` in both cases.
- [x] **AC2 — OAuth CSRF state uses `crypto/rand`.**
  - `internal/auth/google.go`'s `GenerateState` no longer imports
    `math/rand`; it imports `crypto/rand` and `math/big`, and builds each
    of the 32 characters via `crypto/rand.Int(rand.Reader,
    big.NewInt(int64(len(charset))))`.
  - Existing `TestGenerateState`, `TestValidateState`,
    `TestValidateStateExpiry` in `internal/auth/google_test.go` continue
    to pass unchanged (regression, not new coverage for this AC — see
    Context).

## Non-goals

- Threading `config.Config` into `db.go`/`jwt.go` to stop their direct
  `os.Getenv` reads — that's PACK-015.
- `internal/repository/user.go`'s not-found convention — PACK-016.
- Injecting the OIDC provider/verifier into `NewGoogleOAuthManager` so
  `google_test.go` stops making live network calls, and the
  `contains()`/`strings.Contains` cleanup in that same file — PACK-017.
  (This ticket's `google.go` edit is narrowly scoped to `GenerateState`'s
  RNG call; it does not touch `NewGoogleOAuthManager` or add test
  isolation.)
- `UserId`→`UserID` casing, `parseName` duplication, `go.mod` `//
  indirect` cleanup, `validateTemplateItemNotes` rename — PACK-018.
- `doRequest` retrofit for the four pre-existing handler test files —
  PACK-019.
- `requests/*.http` structural rethink — PACK-020. This ticket
  deliberately adds no `.http` file (see Context).
- No changes to the OAuth login/callback redirect logic, token
  generation/validation logic, or any cookie other than `refreshToken`.

## Expected test files

- `config/config_test.go` (**new** — no config tests exist today):
  - `TestLoad_EnvironmentDefaultsToDevelopment` — with `APP_ENV` unset,
    `Load()` (given valid `DATABASE_URL`/JWT secrets) returns
    `cfg.Environment == "development"`.
  - `TestLoad_EnvironmentReadsAppEnv` — with `APP_ENV=production` set,
    `Load()` returns `cfg.Environment == "production"`.
- `internal/handler/auth_handler_test.go` (**modified**):
  - Add `testConfig(environment string) *config.Config` helper (returns
    `&config.Config{Environment: environment}`).
  - Update all 8 `NewAuthHandler(userRepo, oauthMgr, nil)` call sites to
    `NewAuthHandler(userRepo, oauthMgr, testConfig("development"))`.
  - Extend `TestGoogleCallback_HappyPath`: assert
    `refreshCookie.Secure == false` and `refreshCookie.SameSite ==
    http.SameSiteLaxMode`.
  - Extend `TestLogout_ClearsCookie`: assert the same two attributes on
    the clearing cookie.
  - New `TestGoogleCallback_HappyPath_SecureCookieInProduction`: same
    setup as `TestGoogleCallback_HappyPath` but built with
    `testConfig("production")`, asserting `refreshCookie.Secure == true`.
    Covers AC1's production branch, which the dev-mode tests above don't
    exercise.
- `internal/auth/google_test.go`: no new tests (see Context/AC2) —
  existing `TestGenerateState`, `TestValidateState`,
  `TestValidateStateExpiry` must still pass unchanged after the RNG swap.
- No `.http` file (see Non-goals).

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
