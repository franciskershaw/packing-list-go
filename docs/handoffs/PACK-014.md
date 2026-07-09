# PACK-014 — Mid-project review findings (DRAFT — not yet scoped)

> **This is not an implementation-ready handoff doc.** It's the preserved
> raw output of a mid-project codebase review (2026-07-09), captured so
> none of it is lost before the next available slot to work on it. Run
> `/grill-me` against this doc before writing any code against it — that
> session should also decide whether this stays one ticket or splits into
> several. The findings below span security, architecture, test quality,
> and cosmetic cleanup, which may not belong in a single PR. No acceptance
> criteria have been agreed yet; that's the interview's job.

## Origin

The same day PACK-010 shipped, the user asked a separate agent: "I'm
concerned that my relative inexperience with Go is causing me to establish
bad patterns in the project. Would a Go developer who has been writing Go
code for a few years have any major issues with how I've laid out my code,
the approach I'm taking, or the structure of my files across the project?"

That review flagged three concrete issues, all verified accurate against
the code before acting on them, and all fixed in-session the same day
(commits are the user's own, not tracked here):

1. `os.Exit(1)` inside `db.InitDB()` — a library function hard-exiting the
   process instead of returning an error. Fixed: now returns
   `fmt.Errorf("DATABASE_URL not set")`. (Turned out to already be
   unreachable in practice — `config.Load()` validates `DATABASE_URL`
   first and returns its own error before `InitDB` is ever called — but
   the fix is still correct hygiene for any future caller.)
2. `runtime.Caller(0)` used to resolve the migrations directory path at
   runtime — works in dev because the source tree is present, breaks for
   any compiled/deployed binary where it isn't. Fixed: `//go:embed
   migrations` + the `iofs` source driver (already available in the
   existing `golang-migrate/v4` dependency, no new module needed). Proven
   by building a binary and running it from `/tmp` with no source tree
   present — migrations still applied correctly.
3. `TemplateRepository.UpdateTemplate` always returned `items: []`, even
   when the template had real items attached, because it never re-fetched
   them after the `UPDATE` (unlike `GetTemplateByID`, which does). Fixed to
   match `GetTemplateByID`'s behavior. Regression test
   (`TestUpdateTemplate_PreservesItems` in
   `internal/repository/template_test.go`) written first, confirmed it
   failed against the old code, then the fix made it pass.

The user then asked for a second pass — this time from Claude directly,
not just relaying an external review — covering overall codebase health,
technical debt, and anything else worth flagging before calling the
project "done" for the day. That full pass (every handler, repository, the
auth/JWT/OAuth layer, middleware, migrations, `go.mod`, and test files not
otherwise touched today) is what's captured below. None of it was
implemented — the user asked for it to become a proper backlog entry
instead, worked through the normal process (interview → handoff →
tests → implementation) at a later date.

## Overall assessment (repeated here so it isn't lost)

The architecture is genuinely idiomatic for the areas that are hardest to
teach: narrow, consumer-defined interfaces (`TemplateLookupRepository`,
`ItemLookupRepository`, etc. defined in `handler`, not `repository`), the
`errors.Is(err, sql.ErrNoRows) → nil, nil` pattern keeping HTTP concerns
out of the repository layer, `make([]T, 0)` instead of `nil` slices so
JSON always serializes to `[]`, the `scanX(scan func(...any) error)`
helper pattern unifying `row.Scan`/`rows.Scan`, consistent `%w` error
wrapping, `rows.Err()` checked after every loop. **Nothing found in this
pass is an emergency** — this is a "worth doing deliberately" list, not a
fire-alarm list.

## Findings

### Worth doing soon (the only two with real security relevance)

1. **Refresh-token cookie missing `Secure`/`SameSite`.**
   `internal/handler/auth_handler.go:93`:
   ```go
   c.SetCookie("refreshToken", refreshToken, 7*24*60*60, "/", "", false, true)
   ```
   The `false` is the `secure` flag. A 7-day-lived refresh token cookie
   should be `Secure` (HTTPS-only) once this isn't running on plain
   `localhost`, and `SameSite` should be set explicitly (gin needs
   `c.SetSameSite(...)` called before `SetCookie`) rather than left to
   browser defaults.

2. **OAuth CSRF state uses `math/rand`, not `crypto/rand`.**
   `internal/auth/google.go:64-78` (`GenerateState`) builds the 32-char
   anti-CSRF state token from `math/rand`, which is predictable, not
   cryptographically secure. Low exploitation likelihood given the
   10-minute expiry and one-time-use enforced by `ValidateState`, but the
   whole point of this value is being hard to guess.

### Worth knowing about (real, but not urgent — no live bug today)

3. **`config.Config` is only partially threaded through.**
   `config.Load()` validates `DATABASE_URL`, `JWT_SECRET_ACCESS`, and
   `JWT_SECRET_REFRESH` into `cfg`, but `cfg.DatabaseURL` is never actually
   read anywhere afterward — `db.InitDB()` independently re-reads
   `os.Getenv("DATABASE_URL")` itself (in both `InitDB` and
   `runMigrations`). Same story for `internal/auth/jwt.go`:
   `generateToken`/`validateToken` call
   `os.Getenv("JWT_SECRET_ACCESS"/"_REFRESH")` directly rather than
   accepting the secret as a parameter. Same root pattern as the
   `os.Exit`-in-a-library issue already fixed — implicit global env-var
   reads instead of explicit dependency injection, one layer removed. It's
   why `internal/auth/jwt_test.go` has to `os.Setenv` in an `init()` as a
   workaround rather than constructing things cleanly.

4. **`internal/auth/google_test.go` makes real network calls to Google.**
   Every test in that file calls `NewGoogleOAuthManager`, which calls
   `oidc.NewProvider(ctx, "https://accounts.google.com")` — an actual
   HTTPS call to Google's OIDC discovery endpoint. These are "unit" tests
   with a live third-party network dependency: slow, and they'll fail (for
   reasons unrelated to this code) in any network-restricted CI
   environment, or if Google's endpoint hiccups. Would need the
   provider/verifier injected rather than constructed fresh inside
   `NewGoogleOAuthManager` every time.

5. **`internal/repository/user.go` doesn't follow the "not found → `nil,
   nil`" convention the rest of the codebase uses.**
   Every other repo (`category`, `item`, `template`, `packing_list`)
   checks `errors.Is(err, sql.ErrNoRows)` and returns `(nil, nil)`.
   `user.go`'s `GetUserByID`/`getUserByGoogleID` instead wrap everything
   (including "not found") into `fmt.Errorf("user not found: %w", err)`.
   Works today by accident — `%w` preserves the chain for the one caller
   (`GetOrCreateUser`) that checks `errors.Is` against the wrapped error —
   but it means `AuthHandler.RefreshToken` returns `401` even for a
   genuine DB connectivity failure in `GetUserByID`, not just a missing
   user, and it's an inconsistent pattern in a codebase that's otherwise
   disciplined about exactly this distinction.

### Cosmetic / opportunistic cleanup

6. **`UserId`/`userId` casing** (`internal/auth/jwt.go`,
   `internal/middleware/auth.go:36`, `internal/handler/context.go`) should
   be `UserID`/`userID` per Go's initialism convention — used consistently
   within that subsystem, but everywhere else in the codebase (models,
   repos, handlers) correctly uses `UserID`. Purely cosmetic, but exactly
   the kind of tell an experienced Go reviewer would flag.

7. **`parseName` duplication across handlers.** `category_handler.go`
   factors out a shared `parseName` helper for its Create/Update
   body-binding + validation; `item_handler.go` and `template_handler.go`
   each inline the same ~5-line bind-then-`validateName` block separately
   instead of reusing it. Same class of drift flagged in `LESSONS.md`'s
   2026-07-08 (PACK-008) entry — it crept back in since then, just in a
   different spot.

8. **Hand-rolled `contains()` in `google_test.go`** duplicates
   `strings.Contains` from the standard library.

9. **`go.mod` marks every dependency `// indirect`**, including ones
   imported directly everywhere (`gin`, `uuid`, `jwt/v5`, `lib/pq`,
   `golang-migrate`, `testify`). A clean `go mod tidy` normally leaves
   direct dependencies uncommented. Doesn't affect the build, just muddies
   "what's actually a direct dependency" at a glance.

### Explicitly reviewed and dismissed (not worth doing, recorded so it isn't re-litigated)

- Redundant `userIDFromCtx` guard at the top of every handler, even though
  `AuthMiddleware()` already guarantees it's set — harmless defensive
  check.
- `fmt.Println` logging in `db.go` instead of a real logger — fine until a
  proper logger (e.g. stdlib `slog`) gets introduced project-wide.
- Global `var DB *sql.DB` — pragmatic given every repository already takes
  `*sql.DB` as an explicit constructor parameter; `main.go` is the only
  consumer of the global.
- The two-query (`excludeID` present/absent) shape of
  `TemplateNameExistsForUser`/`CategoryNameExistsForUser`/etc. could be one
  query with `$3::uuid IS NULL OR id != $3`, but the current form is
  perfectly readable.
- `scripts/gen_token.go`'s `//go:build ignore` tag, production guard, and
  `os.Exit` usage are all correct for what that file is (a dev-only script
  entrypoint, not library code).

## Suggested next step

Run `/grill-me` against this doc. Likely questions for that session:
whether items 1-2 (security) ship as their own small ticket separate from
3-9 (architecture/cleanup); whether item 3 (config threading) is worth
doing now or deferred until more of the app exists to make the DI payoff
clearer; whether items 6-9 are worth a dedicated ticket or should ride
along with whatever ticket touches those files next.
