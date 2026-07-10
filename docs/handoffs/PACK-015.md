# PACK-015 — Thread config.Config through db.go and jwt.go

## Context

Part of Epic 6 (Codebase Health & Hardening). Source finding: item 3 in
`docs/handoffs/epic-6-findings.md` — `config.Load()` validates
`DATABASE_URL`/`JWT_SECRET_ACCESS`/`JWT_SECRET_REFRESH` into `cfg`, but
`db.InitDB`/`runMigrations` and `internal/auth/jwt.go` independently
re-read the same env vars via `os.Getenv` instead of accepting the parsed
config. It's why `internal/auth/jwt_test.go` has to `os.Setenv` in an
`init()` as a workaround rather than constructing things cleanly.

Key decisions from the interview:

- **Parameter threading, not a new struct.** Both `db.InitDB` and every
  jwt.go function get the relevant secret/URL as an explicit string
  parameter, rather than introducing a `JWTManager`-style struct. Matches
  the finding's literal wording ("accepting the secret as a parameter")
  and keeps `db`/`internal/auth` decoupled from the `config` package —
  callers that already hold `*config.Config` (like `AuthHandler`, or
  `main.go`) just pass the field through.
- **`db.InitDB(databaseURL string) error`** — the connection string is
  passed in and threaded internally into `runMigrations` too (which had
  its own separate `os.Getenv("DATABASE_URL")` call, now removed as a
  side effect of the parameter threading).
- **jwt.go duplication found and consolidated in the same pass.**
  `GenerateRefreshToken`/`ValidateRefreshToken` never actually routed
  through the private `generateToken`/`validateToken` helpers that
  `GenerateAccessToken`/`ValidateAccessToken` use — each had its own
  separate hand-written implementation, and `TokenType.RefreshToken` was
  dead code (nothing ever called `generateToken` with it). Since every one
  of these function bodies has to change anyway to remove `os.Getenv`,
  consolidated into two shared low-level helpers instead of leaving the
  duplication in place for PACK-018 to find later:
  - `signToken(claims jwt.Claims, secret string) (string, error)` — shared
    by `GenerateAccessToken` and `GenerateRefreshToken`.
  - `parseToken(tokenString, secret string, claims jwt.Claims) error` —
    shared by `ValidateAccessToken` and `ValidateRefreshToken`, wrapping
    the HMAC-signing-method check both used to duplicate.
  - `TokenType`/`AccessToken`/`RefreshToken`/old `generateToken`/
    `validateToken` are removed entirely (confirmed unreferenced outside
    `jwt.go` before removal).
- **New function signatures**:
  `GenerateAccessToken(email, userID, secret string) (string, error)`,
  `GenerateRefreshToken(userID, secret string) (string, error)`,
  `ValidateAccessToken(tokenString, secret string) (*CustomClaims, error)`,
  `ValidateRefreshToken(tokenString, secret string) (*jwt.RegisteredClaims, error)`.
- **`middleware.AuthMiddleware(secret string) gin.HandlerFunc`** — it
  calls `auth.ValidateAccessToken` internally, so it needs the access
  secret too. `main.go` calls it as
  `middleware.AuthMiddleware(cfg.JWTSecretAccess)`.
- **Shared test-secret constants in `testutil`**, to avoid the literal
  `"test-secret-access"` string being duplicated across
  `testutil.AuthHeader` and the 4 handler test files that wire up
  `middleware.AuthMiddleware(...)` (`category_handler_test.go`,
  `item_handler_test.go`, `template_handler_test.go` —
  `template_item_handler_test.go` reuses the same router setup —, and
  `packing_list_handler_test.go`). New exported
  `testutil.TestJWTSecretAccess` / `testutil.TestJWTSecretRefresh`
  constants are the single source of truth; `AuthHeader` uses the former
  directly instead of `os.Setenv` + relying on `jwt.go` reading it back
  out. `auth_handler_test.go`'s `testConfig(environment string)` helper
  (from PACK-014) is extended to also set `JWTSecretAccess`/
  `JWTSecretRefresh` to these constants.
- **`scripts/gen_token.go`** keeps reading `JWT_SECRET_ACCESS` via a
  direct `os.Getenv` call (unchanged style — it already reads
  `DATABASE_URL` the same way, not through `config.Load()`), just passes
  that value into the new `GenerateAccessToken` signature. Not migrated to
  `config.Load()` — that would be scope beyond this ticket's finding.
- **`internal/auth/jwt_test.go`**: the `init()`-based `os.Setenv` workaround
  is removed entirely, replaced with local literal test-secret constants
  passed directly into each call — this was explicitly named in the
  master spec as one of this ticket's goals.
- **`CustomClaims.UserId` casing is untouched** — the `UserId`→`UserID`
  rename is PACK-018's scope, not this ticket's, even though this ticket
  touches the same struct.
- **No new `.http` file.** Zero API-visible behavior change (same
  endpoints, same request/response shapes) — this is purely an
  env-var-to-parameter refactor. Manual verification is a one-time local
  run instead: start the server and hit `GET /health` after AC1 (proves
  `db.InitDB(cfg.DatabaseURL)` still connects/migrates), then a real
  Google login + one authenticated request (e.g. `GET /categories`) after
  AC2 (proves the JWT secret threading and `AuthMiddleware` wiring still
  works end-to-end). No new `requests/*.http` file — consistent with
  PACK-014's precedent of deferring `.http` structure work to PACK-020.

## Acceptance criteria

- [x] **AC1 — `db.InitDB` stops reading `DATABASE_URL` via `os.Getenv`.**
  - `InitDB(databaseURL string) error` — empty string still returns
    `fmt.Errorf("DATABASE_URL not set")` (same guard, now on the
    parameter instead of the env var).
  - `runMigrations` takes the connection string as a parameter from
    `InitDB` instead of calling `os.Getenv("DATABASE_URL")` itself.
  - `main.go` calls `db.InitDB(cfg.DatabaseURL)`.
  - `internal/repository/main_test.go`'s `TestMain` calls
    `db.InitDB(os.Getenv("DATABASE_URL"))` (reusing the value it already
    reads for its own skip-check).
  - Manual check: `go run .` locally with a valid `.env`, confirm
    `GET /health` returns 200 (proves DB connects and migrations run via
    the threaded parameter).
- [x] **AC2 — `jwt.go` stops reading JWT secrets via `os.Getenv`; Access/
  Refresh duplication consolidated.**
  - All four public functions accept the secret as an explicit parameter
    (signatures above); `generateToken`/`validateToken`/`TokenType` are
    removed, replaced by shared `signToken`/`parseToken` helpers used by
    both Access and Refresh paths.
  - `AuthHandler` (`auth_handler.go`) passes `h.cfg.JWTSecretAccess` /
    `h.cfg.JWTSecretRefresh` at each of its 4 call sites.
  - `middleware.AuthMiddleware(secret string) gin.HandlerFunc`; `main.go`
    wires it as `middleware.AuthMiddleware(cfg.JWTSecretAccess)`.
  - `testutil.AuthHeader` uses the new `testutil.TestJWTSecretAccess`
    constant instead of `os.Setenv`.
  - `scripts/gen_token.go` reads `JWT_SECRET_ACCESS` via `os.Getenv` and
    passes it through.
  - `internal/auth/jwt_test.go`'s `init()` `os.Setenv` workaround is gone.
  - Manual check: `go run .` locally, complete a real Google login via
    browser, then call an authenticated endpoint (e.g. `GET /categories`)
    with the returned access token — confirms token generation,
    validation, and `AuthMiddleware` all work with the threaded secrets.

## Non-goals

- `internal/repository/user.go`'s not-found convention — PACK-016.
- OAuth test isolation / live network calls in `google_test.go` — PACK-017.
- `UserId`→`UserID` casing (including on `CustomClaims`), `parseName`
  duplication, `go.mod` `// indirect` cleanup,
  `validateTemplateItemNotes` rename — PACK-018.
- `doRequest` retrofit for the four pre-existing handler test files —
  PACK-019.
- `requests/*.http` structural rethink — PACK-020. This ticket adds no
  `.http` file (see Context).
- Migrating `scripts/gen_token.go` to use `config.Load()` instead of its
  own `os.Getenv` calls.
- Introducing a `JWTManager` struct or any other DI-container-style
  refactor beyond parameter threading.
- Any change to the OAuth login/callback flow, token expiry durations, or
  claims shape (`CustomClaims`'s fields, `jwt.RegisteredClaims` usage).

## Expected test files

- `db/db_test.go` (**new**): `TestInitDB_EmptyDatabaseURL` — asserts
  `InitDB("")` returns an error, without needing a real DB connection
  (regression-preserving coverage for the parameter-based guard that
  replaces the old env-var-based one).
- `internal/repository/main_test.go` (**modified**, no new tests): update
  the one `db.InitDB()` call site to `db.InitDB(os.Getenv("DATABASE_URL"))`.
- `internal/auth/jwt_test.go` (**modified**):
  - Remove the `init()` `os.Setenv` block; add local unexported test
    secret constants (e.g. `testAccessSecret`, `testRefreshSecret`).
  - Update `TestGenerateAccessToken`/`TestRefreshToken` to pass secrets
    directly to each call.
  - New `TestGenerateAccessToken_EmptySecret`,
    `TestGenerateRefreshToken_EmptySecret`,
    `TestValidateAccessToken_EmptySecret`,
    `TestValidateRefreshToken_EmptySecret` — regression guards proving the
    "secret key not set" check still fires now that it's checking a
    parameter instead of an env var; not new business behavior, just
    coverage moved from an implicit (env-var-dependent) guarantee to an
    explicit, directly-testable one.
- `internal/testutil/auth.go` (**modified**, no test file itself but
  affects every handler test that calls `AuthHeader`): add
  `TestJWTSecretAccess`/`TestJWTSecretRefresh` exported constants;
  `AuthHeader` uses the former instead of `os.Setenv`.
- `internal/handler/auth_handler_test.go` (**modified**): extend
  `testConfig(environment string)` to also set `JWTSecretAccess`/
  `JWTSecretRefresh` from the `testutil` constants; update
  `TestRefreshToken_ValidCookie`'s `auth.GenerateRefreshToken` call to
  pass `testutil.TestJWTSecretRefresh`.
- `internal/handler/category_handler_test.go`,
  `item_handler_test.go`, `template_handler_test.go`,
  `packing_list_handler_test.go` (**modified**, no new tests): update the
  `middleware.AuthMiddleware()` call to
  `middleware.AuthMiddleware(testutil.TestJWTSecretAccess)`.
- No `.http` file (see Non-goals / Context).

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
