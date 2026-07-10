# PACK-016 — Fix user.go's not-found convention

## Context

Part of Epic 6 (Codebase Health & Hardening). Source finding: item 5 in
`docs/handoffs/epic-6-findings.md` — every other repository
(`category`, `item`, `template`, `packing_list`) checks
`errors.Is(err, sql.ErrNoRows)` and returns `(nil, nil)` for "not found."
`internal/repository/user.go`'s `GetUserByID`/`getUserByGoogleID` instead
wrap everything — including "not found" — into
`fmt.Errorf("user not found: %w", err)`. Works today by accident (`%w`
preserves the chain for the one caller that checks `errors.Is` against
the wrapped error), but it means `AuthHandler.RefreshToken` returns `401`
even for a genuine DB connectivity failure in `GetUserByID`, not just a
missing user.

Key decisions from the interview:

- **`GetUserByID`/`getUserByGoogleID` adopt the standard convention**:
  `if errors.Is(err, sql.ErrNoRows) { return nil, nil }`, else
  `fmt.Errorf("failed to get user by id: %w", err)` /
  `fmt.Errorf("failed to get user by google id: %w", err)` — matching
  `category.go`'s `GetCategoryByID` precedent exactly (checked at
  `internal/repository/category.go:59-70` at time of writing).
- **`GetOrCreateUser`'s control flow changes accordingly.** It currently
  branches on `errors.Is(err, sql.ErrNoRows)` from `getUserByGoogleID` to
  decide "fall through to create" vs. "user exists." Once
  `getUserByGoogleID` returns `(nil, nil)` for not-found, that branch
  moves to checking `user != nil` instead — the only genuine `err` path
  left is a real DB failure, which now unconditionally wraps and returns
  rather than being conflated with "not found."
- **This is the actual bug fix, not just a signature change**:
  `AuthHandler.RefreshToken` currently returns `401 "user not found"` for
  *any* error from `GetUserByID`, including a genuine DB outage. After
  this ticket, it distinguishes `user == nil` (`401 "user not found"`,
  the real not-found case) from `err != nil` (`500 "failed to look up
  user"`, matching every other handler's convention for repo failures —
  e.g. `category_handler.go`'s `"failed to fetch category"` at 500).
- **`UserRepository` interface signature is unchanged** — still
  `GetUserByID(ctx, userID) (*models.User, error)`. Only the *meaning* of
  a `(nil, nil)` return changes (previously impossible for "not found";
  now the expected shape).
- **No `.http` file.** The genuine-DB-failure branch (the actual fix)
  isn't practically triggerable by hand without killing the DB
  mid-request; the happy path is already covered by any existing manual
  login flow. Consistent with PACK-014/015's precedent of deferring
  `.http` work to PACK-020.

## Acceptance criteria

- [x] **AC1 — `user.go` repo methods return `(nil, nil)` for not-found.**
  - `GetUserByID` and `getUserByGoogleID` both check
    `errors.Is(err, sql.ErrNoRows)` and return `(nil, nil)`; any other
    error is wrapped with `fmt.Errorf("failed to get user by id: %w", err)`
    / `fmt.Errorf("failed to get user by google id: %w", err)`
    respectively.
  - `GetOrCreateUser` branches on `user != nil` (from `getUserByGoogleID`)
    instead of `errors.Is(err, sql.ErrNoRows)`; a genuine error from
    `getUserByGoogleID` always short-circuits with
    `fmt.Errorf("failed to look up user: %w", err)` (unchanged wording,
    now unconditional rather than conditional on the old convention).
  - Existing `internal/repository/user_test.go`'s
    `TestGetUserByID_NotFound` updated: asserts `NoError` + `Nil(user)`
    instead of `Error`.
- [x] **AC2 — `AuthHandler.RefreshToken` distinguishes not-found from a
  real lookup failure.**
  - `user == nil` (no error) → `401` with `{"error": "user not found"}`
    (unchanged response for the genuine not-found case).
  - `err != nil` → `500` with `{"error": "failed to look up user"}` (new
    — previously indistinguishable from not-found).

## Non-goals

- No change to `category`/`item`/`template`/`packing_list` repositories —
  they already follow the convention this ticket is bringing `user.go`
  in line with.
- No change to the `UserRepository` interface signature
  (`internal/handler/auth_handler.go`).
- OAuth test isolation / live network calls in `google_test.go` — PACK-017.
- `UserId`→`UserID` casing, `parseName` duplication, `go.mod` `//
  indirect` cleanup, `validateTemplateItemNotes` rename — PACK-018.
- `doRequest` retrofit for the four pre-existing handler test files —
  PACK-019.
- `requests/*.http` structural rethink — PACK-020. This ticket adds no
  `.http` file (see Context).
- No change to `GoogleCallback`'s handling of `GetOrCreateUser` errors
  (already returns 500 generically, untouched by this ticket).

## Expected test files

- `internal/repository/user_test.go` (**modified**, no new tests beyond
  the corrected assertion):
  - `TestGetUserByID_NotFound` updated to assert `(nil, nil)` instead of
    an error — the existing test currently encodes the old, incorrect
    convention.
  - `TestGetOrCreateUser_CreatesNewUser` and
    `TestGetOrCreateUser_ReturnsExistingAndUpdatesLastLogin` are existing
    regression coverage for `GetOrCreateUser`'s control-flow refactor —
    both paths (create-on-not-found, return-existing-and-update) must
    still pass unchanged; no new test needed since these already
    exercise both branches of the changed `if user != nil` check.
  - No new repo-level test for a genuine DB failure — impractical against
    a real Neon DB in an integration test, consistent with how no other
    repo tests that path either.
- `internal/handler/auth_handler_test.go` (**modified**): two new tests —
  - `TestRefreshToken_UserNotFound`: `MockUserRepository.GetUserByID`
    returns `(nil, nil)`, asserts `401` and
    `{"error": "user not found"}`.
  - `TestRefreshToken_UserLookupError`: `MockUserRepository.GetUserByID`
    returns `(nil, errors.New("db error"))`, asserts `500` and
    `{"error": "failed to look up user"}`. This is the test that would
    have failed against the old code (old code returned 401 for this
    case too) — the concrete regression guard for the bug this ticket
    fixes.
- No `.http` file (see Non-goals / Context).

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
