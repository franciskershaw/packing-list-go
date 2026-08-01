# PACK-031 — `users` NOT NULL defaults for `avatar_url`, `display_name`, `last_login_at`

## Context

Filed 2026-07-14 closing out PACK-030: `GET /me` 500'd against the dev
token because `scripts/gen_token.go`'s user upsert originally omitted
`avatar_url`, leaving it genuinely `NULL` — the column is nullable in the
schema (`db/migrations/000001_init_schema.up.sql`) but no legitimate app
code path ever leaves it unset (`GetOrCreateUser`'s `avatarURL` param and
`IDTokenClaims.AvatarURL`, `internal/auth/google.go:26`, are both plain
`string`, defaulting to `""` not absence). `GetUserByID` scans the column
directly into a non-nullable Go `string`
(`internal/repository/user.go:74-101`), so a genuine `NULL` fails the scan
with a real error, not a clean not-found. The dev-token seed fix itself
already shipped as part of PACK-030 (`gen_token.go` now seeds and
backfills a placeholder `avatarUrl`) — this ticket is the schema-level and
test-coverage follow-up, not a re-fix of that symptom.

**Design-gate finding**: follows precedent, no ADR. Migration shape
(backfill then constrain) mirrors PACK-033's categories migration; the
regression-test technique mirrors `archivePackingListDirect`'s raw-SQL-
fixture pattern (`internal/repository/packing_list_test.go:113`).

**Key decisions from the interview:**

- **Scope expanded to `display_name`, bundled into this ticket.**
  `display_name` (`db/migrations/000001_init_schema.up.sql:8`) has the
  identical structural gap: nullable column, non-nullable Go `string`
  scan target in `GetUserByID`/`getUserByGoogleID`, no real code path
  ever sets it `NULL`. It just hasn't bitten yet by coincidence. Same fix
  pattern, same table, same migration file — bundled rather than filed as
  a separate ticket.
- **The NULL rows aren't hypothetical.** Queried the real dev DB during
  this interview: 3 of 5 `users` rows currently have `avatar_url IS
  NULL` (`display_name` too) — leftover `repo-test-*` fixture rows from
  interrupted test runs (`internal/repository/main_test.go`'s `TestMain`
  inserts a user via raw SQL omitting both columns, deleted in teardown
  only on a clean run). A bare `ALTER COLUMN ... SET NOT NULL` would fail
  outright against this real data — the migration backfills first
  (`UPDATE users SET avatar_url = '' WHERE avatar_url IS NULL`, same for
  `display_name`), then applies `NOT NULL DEFAULT ''` to both. No manual
  cleanup of those 3 rows needed — the backfill step handles them.
- **Down migration is schema-only, not data-reversing.** The backfill is
  inherently lossy (no way to tell a row's `''` was originally `NULL` vs.
  genuinely empty after rollback). Down migration just drops `NOT NULL`/
  `DEFAULT` on both columns — matches `000002`'s down migration precedent
  (drops the index it added, doesn't reverse any data change either).
- **Regression test reframed from how the ticket was originally
  described.** The 2026-07-14 filing said "insert via raw SQL omitting
  `avatar_url` and assert `GetUserByID` handles it" — written before this
  migration existed. Once both columns are `NOT NULL DEFAULT ''`,
  *omitting* them in a raw `INSERT` produces `''`, not `NULL` — there's
  no longer any way to produce a `NULL` at all, which is the point, but
  it also means the originally-described test wouldn't exercise
  anything. The real regression guard: **attempt an explicit `INSERT ...
  avatar_url = NULL` (and same for `display_name`) via raw SQL and assert
  it fails with a constraint-violation error** — proving the DB now
  rejects what previously silently succeeded. A second, smaller test
  locks in the `DEFAULT ''` path itself (omit both columns, insert
  succeeds, `GetUserByID` returns `AvatarURL == ""` and
  `DisplayName == ""`).
- **Scope expanded a second time, to `last_login_at`, found while writing
  the tests themselves.** `last_login_at TIMESTAMPTZ` (`000001`, no
  default, no `NOT NULL`) has the identical gap — scanned into a
  non-nullable `time.Time` in the same three read paths. Found via my own
  `TestInsertUser_OmittedColumnsDefaultToEmptyString` draft: omitting
  `last_login_at` (incidental to that test's actual purpose) reproduced
  the exact scan-error bug. Checked the live dev DB: **all 5 rows**
  currently have `NULL` `last_login_at` — including the real
  `dev-tools-kitted-test-account` row `scripts/gen_token.go` seeds — more
  exposed than the other two columns, since `getUserByGoogleID` (called
  on *every* Google OAuth login, not just new-user creation) would 500
  against any of them. Bundled in rather than filed separately, same as
  `display_name`. Backfill: `UPDATE users SET last_login_at =
  COALESCE(created_at, CURRENT_TIMESTAMP) WHERE last_login_at IS NULL`,
  then `DEFAULT CURRENT_TIMESTAMP` (mirrors `created_at`'s existing
  default) + `NOT NULL`.
- **No changes needed to `internal/repository/user.go`.**
  `GetOrCreateUser`/`GetUserByID`/`getUserByGoogleID` already assume
  non-null values in their scan targets — that assumption becomes true
  once the DB enforces it. This is a pure schema + test-coverage ticket.
- **No manual `.http` verification needed.** No endpoint behavior changes
  from a client's perspective — `GET /me`'s response shape is unchanged,
  and the only behavioral fix (the dev-token 500) already shipped and was
  verified in PACK-030. Manual verification here is applying the
  migration against the real dev DB and confirming it succeeds against
  the live NULL rows found above — proof it's not just a clean-slate-DB
  migration.
- **Implementation note: editing an already-applied migration file
  doesn't get picked up by `InitDB`/`TestMain` on its own.** `last_login_at`
  was added to `000004`'s SQL *after* that version had already been
  applied to the dev DB (from testing the first two columns) — golang-migrate
  tracks applied versions and only runs pending ones forward, so the edit
  was silently inert until the applied `000004` was explicitly rolled back
  (`migrate.Steps(-1)`) and reapplied. Only came up because this is a
  shared, persistent dev DB, not a fresh one per run — worth remembering
  for any future migration edited after its first local apply.

## Acceptance criteria

- [x] Migration `000004_users_not_null_defaults`: backfills existing
      `NULL` `avatar_url`/`display_name` rows to `''` and `last_login_at`
      to `COALESCE(created_at, CURRENT_TIMESTAMP)`, then sets
      `NOT NULL DEFAULT ''` on the two string columns and
      `NOT NULL DEFAULT CURRENT_TIMESTAMP` on `last_login_at`
- [x] Down migration drops `NOT NULL`/`DEFAULT` on all three columns
      (schema only, no data reversal — accepted lossy rollback per the
      decision above)
- [x] Migration applies cleanly against the real dev DB, including its
      current NULL rows across all three columns (manual verification —
      `go run main.go` or the repo test suite's `TestMain` both trigger
      it; required an explicit down+up replay mid-implementation after
      `last_login_at` was added to an already-applied `000004` — see note
      above)
- [x] Integration test: raw-SQL `INSERT` explicitly setting
      `avatar_url = NULL` fails with a constraint-violation error
- [x] Integration test: raw-SQL `INSERT` explicitly setting
      `display_name = NULL` fails with a constraint-violation error
- [x] Integration test: raw-SQL `INSERT` explicitly setting
      `last_login_at = NULL` fails with a constraint-violation error
- [x] Integration test: raw-SQL `INSERT` omitting `avatar_url`,
      `display_name`, and `last_login_at` succeeds, and `GetUserByID` on
      that row returns `AvatarURL == ""`, `DisplayName == ""`, and a
      `LastLoginAt` close to the insert time, with no error

## Non-goals

- No changes to `internal/repository/user.go` or any other Go source —
  see decision above
- No changes to `scripts/gen_token.go` — already seeds real `avatarUrl`/
  `displayName`/`last_login_at` values since PACK-030
- No data-reversing down migration — schema-only rollback, explicitly
  accepted as lossy
- No new/changed `.http` manual-verification section — no client-visible
  behavior change beyond what PACK-030 already shipped and verified

## Expected test files

- `db/migrations/000004_users_not_null_defaults.up.sql` /
  `.down.sql` — new
- `internal/repository/user_test.go` — extended existing file with four
  new tests: `TestInsertUser_NullAvatarURLRejected`,
  `TestInsertUser_NullDisplayNameRejected`,
  `TestInsertUser_NullLastLoginAtRejected`,
  `TestInsertUser_OmittedColumnsDefaultToUsableValues` — all raw-SQL
  fixtures (mirroring `archivePackingListDirect`), since no real
  `GetOrCreateUser` call can ever pass `NULL`
