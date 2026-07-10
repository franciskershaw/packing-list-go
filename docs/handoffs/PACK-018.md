# PACK-018 — Naming & duplication cleanup

## Context

Part of Epic 6 (Codebase Health & Hardening). Source findings: items 6,
7, 9, 11 in `docs/handoffs/epic-6-findings.md`, bundled as "cosmetic /
opportunistic cleanup" — all small, similar-risk.

Key decisions from the interview:

- **Item 7 (`parseName` duplication) turned out different from how it was
  archived.** `category_handler.go`'s `parseName()` only works because
  category's Create/Update body is exactly `{name}`. `item_handler.go`
  and `template_handler.go` have extra fields (`categoryId`,
  `description`), so they structurally can't call `parseName()` as
  written — the name-parsing part is already shared correctly via the
  `validateName` helper all three call directly. What's actually
  duplicated verbatim is the surrounding
  `if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400,
  {"error": "invalid request body"}); return }` block — found in **13
  places across 6 files** (`category_handler.go`, `item_handler.go` x2,
  `template_handler.go` x2, `packing_list_handler.go` x2,
  `packing_list_item_handler.go` x3, `template_item_handler.go` x3), not
  just the 3 files named in the original finding. All 13 are
  byte-identical (same status code, same error message). Decided to fix
  all 13, not just the originally-named 3, since the pattern and fix are
  identical everywhere and leaving 10 sites for a later ticket to
  rediscover isn't worth it.
- **Fix: a generic `bindJSON(c *gin.Context, target any) bool` helper**
  in `internal/handler/validation.go` (the existing shared-helpers file
  for `validateName`/`validateDescription`), wrapping
  `c.ShouldBindJSON(target)` + the 400 response. Works regardless of each
  handler's request struct shape since it takes `any`. `category_handler.go`'s
  now-redundant `parseName()` wrapper is deleted; its 2 call sites switch
  to `bindJSON` + `validateName` directly, matching the pattern
  `item_handler.go`/`template_handler.go` already use.
- **No existing test covers a malformed-JSON-body request anywhere in the
  handler package** — checked directly, zero hits for
  `"invalid request body"` in any `_test.go` file. Since `bindJSON` is a
  new shared helper, one representative black-box test is added
  (`category_handler.go`'s `Create`, chosen arbitrarily as one of the 13
  call sites) rather than duplicating the same assertion 13 times for
  logic that's now provably identical everywhere.
- **Item 6 (`UserId`/`userId` casing)**: renames both the Go identifiers
  *and* the JWT's `json:"userId"` struct tag to `userID` — i.e. the
  wire format of the JWT claims payload changes, not just the Go code.
  Confirmed acceptable since there's no production deployment yet (any
  already-issued token, including a 7-day refresh token, is only ever a
  short-lived dev/test token). Also renames the `gin.Context` string key
  `"userId"` → `"userID"` in `middleware/auth.go`/`context.go` for
  consistency — this key is process-internal only (never serialized),
  zero compatibility risk either way.
- **Item 9 (`go.mod` `// indirect` cleanup)**: run `go mod tidy` to let
  Go correctly classify direct vs. indirect dependencies. Purely
  mechanical bookkeeping — diff will be reviewed before presenting as
  done to confirm nothing beyond indirect-marker changes occurred (no
  version bumps, no removals).
- **Item 11 (`validateTemplateItemNotes` rename)**: renamed to
  `validateItemNotes` in `template_item_handler.go`, updating its 4 call
  sites (2 in `template_item_handler.go`, 2 in
  `packing_list_item_handler.go`).
- **No manual verification / `.http` file.** All four changes are
  internal refactors (identifier renames, a shared-helper extraction with
  byte-identical output, dependency-file bookkeeping) with zero
  API-visible behavior change.

## Acceptance criteria

- [x] **AC1 — `UserId`/`userId` → `UserID`/`userID`.**
  - `internal/auth/jwt.go`: `CustomClaims.UserId` field → `UserID`,
    struct tag `json:"userId"` → `json:"userID"`, function parameter
    names `userId` → `userID` in `GenerateAccessToken`/
    `GenerateRefreshToken`.
  - `internal/middleware/auth.go:36`: `c.Set("userId", claims.UserId)` →
    `c.Set("userID", claims.UserID)`.
  - `internal/handler/context.go`: `c.Get("userId")` → `c.Get("userID")`,
    doc comment updated.
  - `internal/auth/jwt_test.go`: `claims.UserId` → `claims.UserID`.
- [x] **AC2 — Shared `bindJSON` helper replaces the 13-site duplicated
  bind-then-400 block.**
  - `internal/handler/validation.go` gets
    `bindJSON(c *gin.Context, target any) bool`.
  - All 13 call sites across `category_handler.go`, `item_handler.go`,
    `template_handler.go`, `packing_list_handler.go`,
    `packing_list_item_handler.go`, `template_item_handler.go` use it.
  - `category_handler.go`'s `parseName()` is deleted; its 2 call sites
    become `bindJSON` + `validateName`.
  - New representative test (see Expected test files) for the malformed-
    JSON-body path, previously untested anywhere.
- [x] **AC3 — `go.mod` dependency classification fixed.**
  - `go mod tidy` run; direct dependencies (`gin`, `uuid`, `jwt/v5`,
    `lib/pq`, `golang-migrate`, `testify`, `oidc`, `oauth2`, `godotenv`,
    etc.) no longer marked `// indirect`.
- [x] **AC4 — `validateTemplateItemNotes` → `validateItemNotes`.**
  - Renamed in `template_item_handler.go`; all 4 call sites
    (`template_item_handler.go` x2, `packing_list_item_handler.go` x2)
    updated.

## Non-goals

- No change to `internal/repository/user.go` — PACK-016 (already done).
- OAuth test isolation — PACK-017 (already done).
- `doRequest` retrofit for the four pre-existing handler test files —
  PACK-019.
- `requests/*.http` structural rethink — PACK-020.
- No change to any handler's actual request/response *shape* — only the
  internal bind-and-validate mechanics.
- No new coverage for `VerifyIDToken`/`ExchangeCodeForToken` (still out
  of scope per PACK-017's non-goals).

## Expected test files

- `internal/auth/jwt_test.go` (**modified**, no new tests): `claims.UserId`
  → `claims.UserID` in `TestGenerateAccessToken`.
- `internal/handler/category_handler_test.go` (**modified**): one new
  test — malformed JSON body on `POST /categories` asserts `400` and
  `{"error": "invalid request body"}`. This is the first test anywhere in
  the handler package to exercise that path; representative coverage for
  `bindJSON`, not duplicated across the other 12 call sites since the
  logic is now provably identical.
- No other test files change behaviorally — `item_handler_test.go`,
  `template_handler_test.go`, `packing_list_handler_test.go`,
  `packing_list_item_handler_test.go`, `template_item_handler_test.go`
  all continue to pass unchanged (same responses, different internal
  plumbing).
- No `.http` file (see Non-goals / Context).

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
