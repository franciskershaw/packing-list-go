# Packing List API — Master Spec

> Working title only — "Packing List API" is a placeholder naming
> convention, not a final product name. Avoid introducing a product name
> into code, docs, or endpoint naming until one is chosen.

> This spec was written retroactively (2026-07-07) via the `project-kickoff`
> skill, after Epics 1-3 and part of Epic 4 already had code written against
> them. Epics 1-2 and the "Done" status on early tickets are **reconstructed
> from the existing code and git history**, not from a spec that existed
> before that code was written — see the note at the top of each affected
> handoff doc. Everything from PACK-008 onward follows the process in
> `~/.claude/CLAUDE.md` for real: handoff doc before implementation, tests
> before code.

## Goals

This is a personal packing-list API. A user signs in with Google, builds a
personal library of items organized into categories (on top of a shared set
of system-provided defaults), assembles reusable packing templates from that
library, and generates packing lists for specific trips/events — optionally
seeded from a template — which they tick items off on as they pack.

## Core use cases

- Sign in with a Google account; no separate password/credential system.
- Browse system-provided categories/items and extend them with personal ones.
- Build a named template (e.g. "Weekend hiking") listing items with
  quantity/notes, organized by category.
- Create a packing list for an actual trip, optionally seeded from a
  template, then add/remove/adjust items independently of the template.
- Tick items off as packed while packing; bulk pack/unpack the whole list.
- Archive old lists instead of deleting them.

## Non-goals (current scope)

- No sharing/collaboration on lists, templates, or categories between users.
- No password-based auth — Google OAuth only.
- No mobile/offline sync concerns — this is the API only.

## Key architecture decisions

- **Stack**: Go, Gin, PostgreSQL (Neon), migrations as plain up/down SQL
  files in `db/migrations/`.
- **Auth**: Google OAuth2/OIDC login. Custom JWT access token (15 min,
  bearer header) + refresh token (7 day, httponly cookie). No server-side
  session store or blacklist — logout only clears the refresh cookie; a
  live access token remains valid until it naturally expires. See
  `docs/handoffs/PACK-005.md`.
- **Repository pattern**: repository interfaces are defined in the
  `handler` package (consumer-defined), not in `repository`. Each
  `Postgres*Repository` in `internal/repository` implements the interface(s)
  its handlers need.
- **Handlers**: structs with injected dependencies (repo, and for auth,
  the OAuth manager + config), not standalone functions. Enables mocking
  in `package handler_test` black-box tests via `httptest` + `gin.New()` +
  `testify/mock`.
- **Ownership model**: rows with `user_id IS NULL` are system-level
  (categories, items) and visible to everyone but not editable/deletable by
  users; rows with a `user_id` are owned by that user and scoped to them on
  every read/write. Helpers: `isOwnedBy` / `isOwned` / `isItemOwned` in
  `internal/handler`.
- **Soft delete**: `packing_lists.archived_at` — `DELETE /lists/:id` sets
  it rather than removing the row.
- **DB schema**: all tables for the full backlog below (`categories`,
  `items`, `templates`, `template_items`, `packing_lists`,
  `packing_list_items`) were created up front in
  `db/migrations/000001_init_schema.up.sql`, ahead of the handlers that use
  them.

## Ticket backlog

Status reflects the state of the code as of 2026-07-07.

### Epic 1: Foundations
*(reconstructed — see `docs/handoffs/PACK-001.md`)*

- **PACK-001** — Project scaffold, config loading, DB connection & migrations,
  server bootstrap with a public health check endpoint. **Done.**
- **PACK-002** — JWT helpers: generate/validate access tokens (15 min) and
  refresh tokens (7 day) signed with HS256, separate secrets per token type.
  **Done.**
- **PACK-003** — Auth middleware: extract and validate a Bearer access
  token from the `Authorization` header, reject with 401 if missing/invalid,
  inject `userId`/`email` into the Gin context for downstream handlers.
  **Done.**

### Epic 2: Google Authentication
*(reconstructed — see `docs/handoffs/PACK-004.md` and `PACK-005.md`)*

- **PACK-004** — Google OAuth login flow: `GET /auth/google/login` redirects
  to Google's consent screen with a CSRF state token; `GET
  /auth/google/callback` exchanges the code, verifies the ID token via OIDC,
  gets-or-creates the local user record, issues access + refresh tokens.
  **Done.**
- **PACK-005** — Token refresh and logout: `POST /auth/refresh` reads the
  refresh cookie and issues a new access token; `POST /auth/logout` clears
  the refresh cookie. **Done.**

### Epic 3: Item Library
*(reconstructed — see `docs/handoffs/PACK-006.md` and `PACK-007.md`)*

- **PACK-006** — Category management.
  - `GET /categories` — system categories + the user's own
  - `POST /categories` — create a user-owned category (name must be unique
    per user)
  - `PATCH /categories/:id` — rename, owner only
  - `DELETE /categories/:id` — owner only, blocked (409) if any items exist
    under it
  - **Done.**
- **PACK-007** — Item management.
  - `GET /items?category_id=&search=` — system + user-owned items,
    optionally filtered by category and name search
  - `POST /items` — create a user-owned item under an accessible category
    (name must be unique within that category)
  - `PATCH /items/:id` — owner only, can move category
  - `DELETE /items/:id` — owner only, blocked (409) if referenced by any
    template or packing list
  - **Done.**

### Epic 4: Templates

- **PACK-008** — Template CRUD.
  - `GET /templates` — templates belonging to the user
  - `POST /templates` — create a template (name, optional description)
  - `GET /templates/:id` — template with its items
  - `PATCH /templates/:id` — update name/description
  - `DELETE /templates/:id` — delete
  - **Status: Done.** Rebuilt test-first via `grill-me` and
    `docs/handoffs/PACK-008.md` — the first ticket to go through the new
    pipeline end to end (handoff doc → tests → repository → handler →
    routes wired into `main.go`).
- **PACK-009** — Managing items on a template.
  - `POST /templates/:id/items` — add an item with optional quantity/notes
  - `PATCH /templates/:id/items/:itemId` — update quantity/notes
  - `DELETE /templates/:id/items/:itemId` — remove
  - `POST /templates/:id/items/bulk` — add every item from a given category
  - **Status: Done.** `docs/handoffs/PACK-009.md` — real `template_items`
    join added to `GET /templates/:id`; `GET /templates` (list) still
    returns `items: []` by design.

### Epic 5: Packing Lists

- **PACK-010** — Packing list creation.
  - `POST /lists` — name + optional `event_date`; optional `template_id`
    copies that template's items onto the list at creation time; list is
    owned by the authenticated user
  - **Status: Done.** `docs/handoffs/PACK-010.md` — response includes the
    seeded items flat (grouping by category is PACK-011's job); `sortOrder`/
    `archivedAt` intentionally not exposed yet.
- **PACK-011** — Packing list management.
  - `GET /lists` — active (non-archived) lists for the user
  - `GET /lists?archived=true` — archived lists
  - `GET /lists/:id` — list with items grouped by category
  - `PATCH /lists/:id` — update name/event_date
  - `DELETE /lists/:id` — soft delete via `archived_at`
  - **Status: Done.** `docs/handoffs/PACK-011.md` — `GET /lists` omits
    items entirely (metadata only); `GET /lists/:id` introduces a new
    grouped-by-category response shape (`PackingListDetail`); `archivedAt`
    is not exposed in any response yet (no concrete consumer).
- **PACK-012** — Managing items on a packing list.
  - `POST /lists/:id/items` — add an item
  - `PATCH /lists/:id/items/:itemId` — update quantity/notes/sort order
  - `DELETE /lists/:id/items/:itemId` — remove
  - `POST /lists/:id/items/bulk` — add every item from a category
  - **Status: Done.** `docs/handoffs/PACK-012.md` — mirrors PACK-009's
    template-item endpoints; `sort_order` is now populated (PATCH-only)
    and `GET /lists/:id` (PACK-011) orders by it, NULLS LAST, falling back
    to alphabetical.
- **PACK-013** — Packing/ticking items.
  - `PATCH /lists/:id/items/:itemId` — accepts `is_packed: true/false`
  - `POST /lists/:id/pack-all` — mark every item packed
  - `POST /lists/:id/unpack-all` — reset every item to unpacked
  - **Status: Done.** `docs/handoffs/PACK-013.md` — `pack-all`/`unpack-all`
    return 204 (no body); `isPacked` is `*bool` on the existing PATCH
    endpoint so explicit `false` is distinguishable from omission.

### Epic 6: Codebase Health & Hardening

*(Split via `grill-me` on 2026-07-10 from a single catch-all PACK-014 into
seven tickets, grouped by shared risk/effort/theme. Full technical detail
for every item below lives in `docs/handoffs/epic-6-findings.md`, kept as
the shared reference archive rather than duplicated per ticket — each
ticket's own future `grill-me` should read the numbered item(s) it
references before writing that ticket's real handoff doc. That archive
lived at `docs/handoffs/PACK-014.md` until PACK-014 itself went through
its own `grill-me` on 2026-07-10 and needed that filename for its own
real, implementation-ready handoff doc.)*

- **PACK-014** — Security hardening: refresh-token cookie & OAuth CSRF token.
  - Set `Secure`/`SameSite` on the refresh-token cookie
    (`internal/handler/auth_handler.go`).
  - Switch OAuth CSRF state generation from `math/rand` to `crypto/rand`
    (`internal/auth/google.go`).
  - **Status: Done.** See `docs/handoffs/PACK-014.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` items 1-2 (source findings).
- **PACK-015** — Thread `config.Config` through `db.go` and `jwt.go`.
  - Stop re-reading `DATABASE_URL`/`JWT_SECRET_ACCESS`/`JWT_SECRET_REFRESH`
    via `os.Getenv` inside `db.InitDB`/`runMigrations` and
    `internal/auth/jwt.go`; accept parsed config explicitly instead.
  - Removes the `os.Setenv`-in-`init()` workaround in
    `internal/auth/jwt_test.go`.
  - **Status: Done.** See `docs/handoffs/PACK-015.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` item 3 (source finding).
- **PACK-016** — Fix `user.go`'s not-found convention.
  - `internal/repository/user.go`'s `GetUserByID`/`getUserByGoogleID` adopt
    the `errors.Is(sql.ErrNoRows) → nil, nil` convention every other repo
    uses, instead of wrapping "not found" into an error indistinguishable
    from a genuine DB failure.
  - **Status: Done.** See `docs/handoffs/PACK-016.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` item 5 (source finding).
- **PACK-017** — OAuth test isolation.
  - Inject the OIDC provider/verifier into `NewGoogleOAuthManager` so
    `internal/auth/google_test.go` stops making live network calls to
    Google's discovery endpoint.
  - While touching that file: replace its hand-rolled `contains()` helper
    with `strings.Contains`.
  - **Status: Done.** See `docs/handoffs/PACK-017.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` items 4, 8 (source findings).
- **PACK-018** — Naming & duplication cleanup.
  - `UserId`/`userId` → `UserID`/`userID` casing (`internal/auth/jwt.go`,
    `internal/middleware/auth.go`, `internal/handler/context.go`).
  - Extract/reuse a shared `parseName` helper in `item_handler.go`/
    `template_handler.go`, matching `category_handler.go`'s existing
    pattern.
  - `go.mod`: uncomment direct dependencies mismarked `// indirect`.
  - Rename `validateTemplateItemNotes` → `validateItemNotes`, update both
    call sites (`template_item_handler.go`, `packing_list_item_handler.go`).
  - **Status: Done.** See `docs/handoffs/PACK-018.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` items 6, 7, 9, 11 (source findings).
- **PACK-019** — Handler test `doRequest` retrofit.
  - Retrofit `category_handler_test.go`, `template_handler_test.go`,
    `item_handler_test.go`, `template_item_handler_test.go` to use the
    shared `doRequest` helper (already used by every packing-list test
    file) instead of the repeated `httptest.NewRequest`+headers+
    `ServeHTTP` block.
  - **Status: not started.** See `docs/handoffs/epic-6-findings.md` item 10.
- **PACK-020** — `requests/*.http` structural rethink.
  - Now unblocked — every feature ticket touching `.http` files (Epics
    1-5) is done. Needs its own `grill-me`: decide whether one file can
    serve both "quick per-commit spot check" and "full manual regression
    pass," or whether those are different enough needs to warrant
    splitting (e.g. separate smoke-test vs. regression files), and whether
    the `.http`-per-resource convention itself should be reconsidered.
  - **Status: not started.** See `docs/handoffs/epic-6-findings.md` item 12.
