# Epic 6 findings archive (reference for PACK-014 – PACK-020)

> **This is not an implementation-ready handoff doc — it's a shared
> reference archive.** It's the preserved raw output of a mid-project
> codebase review (2026-07-09), originally captured as a single draft
> "PACK-014". On 2026-07-10, a `grill-me` session split it into seven
> tickets (`PACK-014`–`PACK-020` in `docs/specs/master-spec.md`), each
> covering a numbered subset of the findings below, grouped by shared
> risk/effort/theme. This file is kept as-is rather than duplicated seven
> times — **before writing any of those tickets' own real handoff docs,
> run `/grill-me` for that specific ticket and read the item numbers it
> references here first.** No acceptance criteria are agreed for any of
> them yet; that's each ticket's own interview's job.
>
> **Moved 2026-07-10** from `docs/handoffs/PACK-014.md` to this filename
> once PACK-014 itself went through its own `grill-me` and needed that
> filename for its own real, implementation-ready handoff doc. This file's
> content is unchanged by the move — items 3–12 are still the source
> material for PACK-015–020's own future `grill-me` sessions.

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

10. **Handler test files repeat an identical HTTP-request-construction
    block instead of sharing a helper.** Found during PACK-011's retro
    (2026-07-10): `httptest.NewRequest(...)` + `Content-Type` +
    `Authorization` header + `httptest.NewRecorder()` + `r.ServeHTTP(...)`
    appears verbatim ~150+ times across `category_handler_test.go` (24),
    `template_handler_test.go` (30), `item_handler_test.go` (37),
    `template_item_handler_test.go` (38), and `packing_list_handler_test.go`
    (37). A `doRequest` helper collapsing this to one call now exists in
    `internal/handler/handler_test_helpers_test.go` (added as part of this
    finding, alongside a `CLAUDE.md` note that new handler test files
    should use it) — but the four pre-existing files above still use the
    old repeated-block style and are the actual retrofit candidate here.
    Same class of drift as item 7 (`parseName` duplication) and the
    2026-07-08 (PACK-008) `LESSONS.md` entry — duplication creeping in
    despite an explicit standing "self-scan for duplication before
    presenting work as done" habit, which is why that habit was tightened
    in this same session to explicitly cover test files, not just
    implementation code.

11. **`validateTemplateItemNotes` is named for templates but also used by
    list items.** Found during PACK-012's `grill-me` (2026-07-10):
    `internal/handler/template_item_handler.go`'s `validateTemplateItemNotes`
    (trims, 200-char limit) is reused as-is by PACK-012's
    `packing_list_item_handler.go` for packing-list item notes, since it's
    already generic in behavior — only its name is template-specific. Not
    renamed in PACK-012 since that would mean touching PACK-009's
    already-shipped file for a purely cosmetic reason, outside PACK-012's
    stated scope. Same class of drift as item 7
    (`parseName` duplication) and item 10 above — a shared helper whose
    name no longer reflects all of its callers. Rename to something
    generic (e.g. `validateItemNotes`) and update both call sites.

12. **`requests/*.http` files need a structural review, not just a tweak —
    deliberately deferred until every feature ticket is done.** Found
    during PACK-012 (2026-07-10): each `.http` file's later sections
    reference `@name`-captured variables from its own Setup/earlier
    sections (e.g. `{{createListWithEventDate.response.body.id}}`), which
    is fine for "run the whole file top-to-bottom once" but makes it
    awkward to isolate and re-run just the section for a single commit —
    you end up re-running everything above it just to repopulate captured
    IDs, even though nothing above changed. Going forward (starting with
    PACK-012's remaining ACs), new sections are self-contained — their own
    throwaway fixtures, no reliance on earlier sections — with an explicit
    marker pointing at what's new for a given commit. The existing
    sections predating this (all of PACK-010/011's, plus PACK-012's
    `POST /lists/:id/items`) were deliberately left as-is rather than
    retrofitted mid-ticket. This item is the reminder to go back once
    every ticket that touches `requests/*.http` is done, and rethink the
    files properly with the full picture visible — including whether one
    file can serve both "quick per-commit spot check" and "full manual
    regression pass" well at once, or whether those are different enough
    needs that they warrant different structures (e.g. splitting
    self-contained smoke-test sections from a comprehensive regression
    pass, or reconsidering the `.http`-per-resource convention itself).
    Not a one-line fix — flagged specifically so it isn't rushed or
    bundled into an unrelated ticket. **Note (2026-07-10, PACK-014
    grill-me):** PACK-014 is explicitly skipping its own manual `.http`
    verification and deferring it here, rather than starting a
    `requests/auth.http` file that this rethink would likely restructure
    anyway.

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

## Suggested next step (resolved 2026-07-10)

This section previously asked whether to split into multiple tickets. That
`grill-me` happened on 2026-07-10 — see `docs/specs/master-spec.md`'s
Epic 6 for the result: seven tickets, each covering a subset of the items
above —

- **PACK-014** — items 1-2 (security)
- **PACK-015** — item 3 (config threading)
- **PACK-016** — item 5 (`user.go` convention)
- **PACK-017** — items 4, 8 (OAuth test isolation, folded together since
  they touch the same file)
- **PACK-018** — items 6, 7, 9, 11 (naming & duplication cleanup, bundled
  since all are small and similar-risk)
- **PACK-019** — item 10 (`doRequest` retrofit, isolated given its diff size)
- **PACK-020** — item 12 (`.http` structural rethink — now unblocked,
  every feature ticket touching `.http` files is done)

Pick any of these up with its own `/grill-me` when ready; each is scoped
enough in `master-spec.md` to start from, but still needs its own
interview and real handoff doc before implementation.
