# Pack-It API — Master Spec

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
  bearer header, stateless) + refresh token (sliding 7-day expiry,
  httponly cookie). Refresh tokens rotate on every use and are DB-backed
  (`refresh_tokens`, one row per login "family," overwritten in place on
  rotation) — a token presented outside its current/grace-window hash is
  reuse and revokes the whole family; `Logout` revokes server-side too,
  not just cookie-clearing. See `docs/handoffs/PACK-005.md` (original
  model) and `docs/handoffs/PACK-027.md` (rotation/reuse-detection).
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
  - **Status: Done.** See `docs/handoffs/PACK-019.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` item 10 (source finding).
- **PACK-020** — `requests/*.http` structural rethink.
  - Rebuilt the 5-file manual `.http` suite into one consistent,
    low-duplication convention: token/host resolve automatically via
    `.env`/`$dotenv` (no more manual paste), `packing_lists.http` reverted
    from PACK-012's self-contained-sections style back to chained shared
    setup (per-commit isolation is no longer the priority now that every
    feature ticket is done), redundant per-endpoint 401 checks trimmed,
    cleanup consolidated per file, and a new `requests/README.md`
    documents the convention. No smoke-test split built; `auth.http`
    stays out of scope (needs its own design for the browser-driven OAuth
    flow).
  - **Status: Done.** See `docs/handoffs/PACK-020.md` (real handoff) and
    `docs/handoffs/epic-6-findings.md` item 12 (source finding).

### Epic 7: Production Readiness & Frontend Bridge

*(Filed 2026-07-11 from an external LLM audit run at the close of Epic 6.
Full detail for every item below lives in
`docs/handoffs/audit-2026-07-11-findings.md`, kept as the shared reference
archive rather than duplicated per ticket — each ticket's own future
`grill-me` should read the numbered item(s) it references there, and
verify the finding against current source, before writing that ticket's
real handoff doc. PACK-029 and PACK-030 below were added later — same day,
separate provenance — see their own entries; both are listed first despite
the higher ticket numbers because they're priority pickups, not because
the numbering implies an order.)*

- **PACK-029** — Unarchive endpoint. **Priority — pick up before
  PACK-021.**
  - `PATCH /lists/:id/unarchive` (or fold `archived: false` into the
    existing `PATCH /lists/:id`) sets `archived_at` back to `NULL`.
    `DELETE /lists/:id` only sets `archived_at`; nothing currently
    reverses it.
  - Found 2026-07-11 during frontend prototyping (not the audit above) —
    the prototype's archive toggle is designed as reversible (tap again
    to restore), which the API can't currently support. Verified against
    current source before filing: confirmed no PATCH/POST route or repo
    method touches `archived_at` in the unarchive direction
    (`internal/repository/packing_list.go`, `main.go` route list).
  - **Status: Done.** Implemented as `POST /lists/:id/unarchive` (see
    `docs/handoffs/PACK-029.md`).
- **PACK-030** — Session-restore / user-profile endpoint. **Priority —
  pick up before PACK-021.**
  - `GET /me` (or similar) returning the authenticated user's profile
    (`id`, `email`, `name`, `avatarUrl`).
  - Found 2026-07-11 during frontend prototyping (not the audit above) —
    `POST /auth/refresh` returns only a new `accessToken`
    (`internal/handler/auth_handler.go`), so a persistent-sign-in flow has
    no way to restore name/avatar without a fresh OAuth login. Also fixes
    a related gap found during verification: even the login response
    (`GET /auth/google/callback`) never returns the avatar URL to the
    client, despite it being captured and stored server-side
    (`GetOrCreateUser`'s `avatarURL` param) — so this endpoint is the
    only way to expose it at all, not just the refresh-time fix.
  - **Status: Done.** See `docs/handoffs/PACK-030.md`. Surfaced a
    follow-up finding, filed below as **PACK-031**.

- **PACK-031** — `avatar_url`/`display_name`/`last_login_at` NOT NULL
  constraints.
  - Found 2026-07-14 closing out PACK-030: `GET /me` 500'd against the
    dev token because `scripts/gen_token.go`'s user upsert omitted
    `avatar_url`, leaving it genuinely `NULL` — the column is nullable in
    the schema (`db/migrations/000001_init_schema.up.sql`) but no
    legitimate app code path ever leaves it unset (`GetOrCreateUser`'s
    `avatarURL` param and `IDTokenClaims.AvatarURL`,
    `internal/auth/google.go:27`, are both plain `string`, defaulting to
    `""` not absence). `GetUserByID` scans the column directly into a
    non-nullable Go `string`, so a genuine `NULL` fails the scan with a
    real error, not a clean not-found.
  - Recommended fix (see PACK-030 close-out retro,
    `LESSONS.md`): migrate `avatar_url` to `NOT NULL DEFAULT ''`, so the
    invariant every real code path already honors is enforced at the DB
    layer — any future raw-SQL writer that forgets the column fails
    loudly at `INSERT`, not silently in an unrelated downstream ticket.
    Pair with a repo-test regression guard mirroring
    `archivePackingListDirect`'s raw-SQL-fixture technique
    (`internal/repository/packing_list_test.go:116`) — insert a user row
    via raw SQL omitting `avatar_url` and assert `GetUserByID` handles it,
    since the normal `GetOrCreateUser` fixture path can never produce a
    `NULL` to test against. The dev-token seed fix itself
    (`scripts/gen_token.go` now seeds and backfills a placeholder
    `avatarUrl`) already shipped as part of PACK-030 — this ticket is the
    schema-level and test-coverage follow-up, not a re-fix of the
    immediate symptom.
  - Scope grew twice during PACK-031's own `grill-me`/implementation:
    `display_name` (identical gap, found during the interview) and
    `last_login_at` (identical gap, found mid-implementation — all 5 live
    dev DB rows had it `NULL`, more exposed than `avatar_url` since
    `getUserByGoogleID` hits it on every returning-user login). Both
    bundled into the same migration rather than filed separately — see
    `docs/handoffs/PACK-031.md` and `LESSONS.md` (2026-08-01).
  - **Status: Done.** See `docs/handoffs/PACK-031.md`.

- **PACK-032** — OAuth callback: redirect to frontend instead of
  returning JSON. **Priority — blocks the frontend's sign-in ticket
  (`packing-list-react`'s `PACKFE-003`).**
  - `GET /auth/google/callback` (`internal/handler/auth_handler.go`)
    currently sets the refresh cookie and renders the access token as
    JSON directly in the browser tab. Change it to redirect to the
    frontend's callback route instead (e.g.
    `http://localhost:5173/auth/callback` in dev — exact prod value
    TBD once a frontend deployment target is chosen), with **no token
    in the redirect URL**. The refresh cookie continues to be set as
    it is today; the frontend mints its own access token client-side by
    calling `POST /auth/refresh` immediately after landing on that
    route.
  - Found 2026-07-17 during the `packing-list-react` project kickoff —
    flagged directly by the 2026-07-11 audit's finding S9 ("the frontend
    integration will need a deliberate redirect design — never put the
    token in a URL"), and formalized as
    `packing-list-react/docs/adr/001-auth-session-model.md`. That ADR
    documents the full handoff flow this ticket needs to support and
    should be read before this ticket's own `grill-me`.
  - The frontend redirect target will need to be configurable (dev vs
    whatever prod origin gets chosen later) rather than hardcoded — a
    concrete AC for this ticket's own handoff doc, not decided here.
  - Related but distinct from **PACK-027** (refresh-token rotation with
    reuse detection) — that ticket is about strengthening the existing
    token issuance, this one is about how tokens reach the frontend at
    all. Both are auth-integration work and reasonable to pick up in the
    same sitting, but they are separate ACs, not one ticket.
  - **Status: Done.** See `docs/handoffs/PACK-032.md`. A separate,
    unrelated bug (`GOOGLE_REDIRECT_URI`/`GOOGLE_REDIRECT_URL` env var
    name mismatch) was found and fixed while verifying Google Cloud
    Console config for this ticket's manual check — not one of this
    ticket's ACs, flagged and fixed with explicit go-ahead.

- **PACK-033** — System categories seed idempotency. **Priority — blocks
  `packing-list-react`'s `PACKFE-003` (Piece 4, `ItemFormModal`).**
  - `db/seeds/categories.sql`'s `ON CONFLICT DO NOTHING` has always been a
    no-op — `categories.name` has no unique constraint for Postgres to
    detect a conflict against, so re-running the seed duplicates rows.
    The dev DB currently has zero categories seeded at all.
  - Found 2026-07-26 during `packing-list-react` frontend work — flagged
    as a deferred non-goal in that project's `PACKFE-003` entry
    ("becomes its own small `packing-list-go` ticket — addressed when it
    actually blocks building/testing against real data"). That moment
    arrived: no category exists to assign a new item to, blocking the
    frontend's item-creation flow end-to-end.
  - Fix: partial unique index on `categories(name) WHERE user_id IS
    NULL` (system rows only — doesn't touch PACK-006's existing
    per-user uniqueness check), migration
    `000002_categories_system_name_unique`. Seed's `ON CONFLICT` clause
    updated to target it.
  - Scoped to categories only — a system-items seed was also mentioned
    in the original deferred note but isn't part of the actual blocker
    (users create their own items regardless of pre-existing system
    ones); left as a separate future item, not bundled in here.
  - **Status: Done.** See `docs/handoffs/PACK-033.md`.
- **PACK-034** — Template list item counts. **Priority — blocks
  `packing-list-react`'s `PACKFE-004` (Piece 6, Templates list/rail
  assembly).**
  - `GetTemplates`' `scanTemplate` helper always sets `Items: []` — only
    `GetTemplateByID` (the detail fetch) populates items, via a second
    query. Confirmed by reading `internal/repository/template.go`'s
    current source. Every Templates-screen screenshot shows an item
    count on each list/rail row, so the list response needs a count
    without needing the full item list.
  - Found 2026-07-26 during `packing-list-react` frontend work — flagged
    in that project's `PACKFE-004` Architecture entry as its own small
    `packing-list-go` ticket, "landing before Piece 6 needs it, not
    necessarily before Piece 1." That moment arrived: Piece 6 is next
    up and has nothing real to render counts from.
  - Fix direction (final shape TBD via this ticket's own grill-me): add
    an `ItemCount int` field (`json:"itemCount"`) to `models.Template`.
    `GetTemplateByID` sets it to `len(items)` once it's already fetched
    them — free, no query change. `GetTemplates` needs a `COUNT` against
    `template_items` per row (a `LEFT JOIN ... GROUP BY` or a correlated
    subquery — pick during implementation) rather than a second
    per-template round trip. No migration expected — this is query
    logic, not a schema change.
  - Grilled 2026-07-26. New pattern, no ADR (judged small enough for the
    handoff doc to carry — see `docs/handoffs/PACK-034.md`): no existing
    repository method anywhere in this codebase populated a
    computed/aggregate field before this. `ItemCount` landed on the
    shared `models.Template` struct (not a separate list-only type —
    unlike `PackingList`/`PackingListDetail`'s split, this is one field,
    not a genuinely different shape). `GetTemplates` uses a correlated
    `COUNT` subquery, not a `LEFT JOIN`/`GROUP BY`. `scanTemplate` stays
    untouched; `GetTemplates` got its own bespoke scan for the extra
    column.
  - **Found during implementation, beyond the handoff doc's original
    scope**: `UpdateTemplate` already fetches items via
    `GetTemplateItems` (missed when the handoff doc said its `ItemCount`
    "may be stale/0, which is fine") — since it's free and already has
    the data, `ItemCount = len(items)` was set there too, with a new
    assertion added to the existing `TestUpdateTemplate_PreservesItems`
    to cover it (nothing tested that path before).
  - Repository layer implemented and committed first (own review pause),
    per this project's layer-by-layer rule, before the handler layer.
    Handler layer needed no code changes — pure JSON pass-through — just
    two new tests (`TestTemplateList_IncludesItemCount`,
    `TestTemplateGetByID_IncludesItemCount`) locking in the wire
    contract.
  - Manual verification: `requests/templates.http` extended with a
    self-contained itemCount section (own setup/cleanup, mirroring
    `template_items.http`'s precedent). Verified live against a running
    server (curl, equivalent to the `.http` file) — `itemCount: 0` before
    attaching an item, `1` on both `GET /templates` (the `COUNT` query)
    and `GET /templates/:id` (`len(items)`) after, cleanup all 204s.
  - **Status: Done.** See `docs/handoffs/PACK-034.md`.
- **PACK-021** — Server lifecycle hardening.
  - `http.Server` timeouts (`ReadHeaderTimeout`/`ReadTimeout`/
    `WriteTimeout`/`IdleTimeout`), graceful shutdown
    (`signal.NotifyContext` + `Shutdown`), Gin release mode from
    `cfg.Environment`.
  - **Follow-up noted 2026-08-01 (from PACK-027's grill-me):** once
    graceful shutdown exists here, a global periodic sweep of expired/
    revoked `refresh_tokens` rows (a `time.Ticker` goroutine, stopped
    cleanly on shutdown) is the "correct" long-term replacement for
    PACK-027's lazy per-user-on-login cleanup — that ticket deliberately
    didn't build a ticker without shutdown coordination to stop it
    against. Small addition to this ticket's scope when picked up, not a
    separate ticket.
  - **Status: Done.** See `docs/handoffs/PACK-021.md`. Source finding:
    `docs/handoffs/audit-2026-07-11-findings.md` items 1-2.
- **PACK-022** — Request-level abuse protection.
  - Rate limiting (global 120/min per IP; `/auth/google/*`+`/auth/logout`
    10/min; `/auth/refresh` its own 30/min), request body size cap
    (1 MB), and config-driven `server.SetTrustedProxies(...)` (was never
    called at all before this ticket).
  - Grilled and implemented 2026-08-01. Real-frontend manual verification
    (not just the documented curl loops) caught two limit-tuning gaps —
    see `docs/handoffs/PACK-022.md` and `LESSONS.md` (2026-08-01) for the
    full account. Filed **PACK-037** as a follow-up (packed-toggle
    request batching), not built in this ticket.
  - **Status: Done.** See `docs/handoffs/PACK-022.md`. Source finding:
    `docs/handoffs/audit-2026-07-11-findings.md` items 3-4.
- **PACK-023** — OAuth state store fix.
  - Unbounded in-memory map on `/auth/google/login` replaced with a
    stateless signed JWT cookie (double-submit pattern) — see
    `docs/handoffs/PACK-023.md` for the full decision trail.
  - **Status: Done.** See `docs/handoffs/PACK-023.md`. Source finding:
    `docs/handoffs/audit-2026-07-11-findings.md` item 5.
- **PACK-024** — CI pipeline + fail-loud DB tests.
  - GitHub Actions running `gofmt`/`go vet`/`golangci-lint`/
    `govulncheck`/`go test`; repository suite fails (not silently skips)
    without `DATABASE_URL` unless an explicit opt-out flag is passed.
    Likely multi-AC (CI secrets, DB access from CI) — a good real
    exercise of `grill-me`'s new-pattern-vs-precedent question once
    picked up.
  - **Status: not started.** See `docs/handoffs/audit-2026-07-11-findings.md`
    item 6 (source finding).
- **PACK-025** — DB indexes migration.
  - FK columns and `user_id` columns have no indexes beyond primary keys.
  - **Status: not started.** See `docs/handoffs/audit-2026-07-11-findings.md`
    item 7 (source finding).
- **PACK-026** — OpenAPI spec.
  - Generate from the existing handlers as the explicit frontend-contract
    bridge artifact. No functional coupling to PACK-025 — may get pulled
    forward ahead of it if frontend work starts first.
  - **Status: not started.** See `docs/handoffs/audit-2026-07-11-findings.md`
    item 8 (source finding).
- **PACK-027** — Refresh token rotation with reuse detection.
  - Deferral condition met: `packing-list-react`'s **PACKFE-002** (Google
    sign-in & session restore) is Done, so this is no longer standalone
    pickup. Grilled 2026-08-01 — see `docs/handoffs/PACK-027.md` for the
    full decision trail (family model, grace window, cleanup approach,
    a flagged `packing-list-react` follow-up for mid-session force-
    sign-out on revocation).
  - **Design reversal during implementation**: the interview's hash-only
    family-lookup decision (no `jti` claim) was reversed after manual
    verification against the real server showed a token more than one
    rotation stale couldn't be traced to any family, so reuse beyond that
    silently 401'd without ever revoking the live family. Fixed by
    embedding a `familyId` claim in the JWT and looking up by ID instead
    of hash — see the handoff doc and `LESSONS.md` (2026-08-01) for the
    full account.
  - **Status: Done.** See `docs/handoffs/PACK-027.md`. Source finding:
    `docs/handoffs/audit-2026-07-11-findings.md` item 9.
- **PACK-028** — Minor security/idiom cleanup.
  - Bundles: `email_verified` check, refresh-flow UUID/subject
    validation + JWT-secrets-distinct startup assertion,
    ownership-in-WHERE-clause defense-in-depth, `context.Request.Context()`
    propagation, ILIKE wildcard escaping, `gen_token.go` prod-DB guard,
    `sslmode=require` check, non-transactional `BulkAddItems`, structured
    logging (explicitly reopens an Epic-6 dismissal — see the findings
    doc), `main()` extraction, naming stutter/inconsistency, `*string`
    dates.
  - **Note:** the "non-transactional `BulkAddItems`" item (finding 14) is
    superseded by **PACK-035**, which deletes `BulkAddItems` entirely
    rather than patching it in place. Drop that item from this bundle when
    PACK-028 is picked up.
  - **Status: not started.** See `docs/handoffs/audit-2026-07-11-findings.md`
    items 10-18 (source findings).
- **PACK-035** — Delta bulk item mutations for lists and templates.
  - Replaces `POST /lists/:id/items/bulk` and
    `POST /templates/:id/items/bulk` (categoryId-only, hardcoded quantity 1,
    non-transactional loop, no real frontend caller) with
    `PATCH /lists/:id/items/bulk` / `PATCH /templates/:id/items/bulk`: a
    transactional delta contract (`{ items: [{ itemId, quantity }] }`,
    `quantity: 0` means remove) so a client can add/update/remove many items
    in one atomic request instead of one request per item.
  - Found 2026-07-28 during `packing-list-react` work: the item-add modal's
    "Done" flow computes a diff and fires one request per changed item
    (`TripAddItemsModal.tsx`/`TemplateAddItemsModal.tsx`), meaning adding
    ~30 items from a category means ~30 HTTP requests.
  - Supersedes PACK-028's "non-transactional `BulkAddItems`" finding (see
    note on PACK-028 above).
  - **Status: Done.** See `docs/handoffs/PACK-035.md`.
- **PACK-036** — `ItemCount`/`PackedCount` on `GetPackingLists`.
  - `models.PackingList` gains `ItemCount`/`PackedCount` (list mode only —
    `PackingListDetail` is unaffected, its `Categories[].Items[]` already
    carries real `IsPacked`), populated via two correlated `COUNT`
    subqueries on `GetPackingLists`, mirroring `GetTemplates`' own
    `ItemCount` fix exactly (own inline scan, not the shared
    `scanPackingList`).
  - Raised 2026-07-29 during `packing-list-react`'s PACKFE-005 Piece 7
    (list/rail assembly) — the trips list/rail needs a per-trip "n of m
    packed" count, the same gap `GetTemplates` had before its own fix.
  - **Status: Done.** See `docs/handoffs/PACK-036.md`.
- **PACK-037** — Extend the bulk item-mutation delta to cover `isPacked`.
  - `PATCH /lists/:id/items/bulk` (PACK-035's delta contract) currently
    only accepts `{itemId, quantity}`. Extend it to also accept
    `isPacked`, and have `packing-list-react`'s packed-checkbox
    (`TripItemRow` → `useUpdateTripItem`) batch rapid toggles through it
    instead of firing one `PATCH .../items/:itemId` per click.
  - Found 2026-08-01 during PACK-022's manual verification: rapidly
    marking a ~30-item list as packed could burn half the (then-60/min,
    now 120/min) global rate limit budget in one ordinary interaction —
    same shape of problem PACK-035 already solved for bulk item-adds,
    just never extended to the packed-toggle. Debouncing the frontend
    click handler wouldn't fix this (it delays requests, doesn't reduce
    their count) — batching through a real bulk contract is the actual
    fix. See `docs/handoffs/PACK-022.md`'s manual-verification section for
    the full account.
  - **Status: not started.**
