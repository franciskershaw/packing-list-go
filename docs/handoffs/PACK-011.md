# PACK-011 — Packing list management

## Context

PACK-010 built `POST /lists` (creation, optionally seeded from a template).
This ticket covers everything else needed to see and manage a list once it
exists: listing active/archived lists, the grouped-by-category detail view,
renaming/re-dating, and soft-delete (archive). Adding/removing/reordering
items on a list (PACK-012) and packing/ticking (PACK-013) remain out of
scope.

Key decisions from the interview:

- **`GET /lists` is metadata-only — no items.** Each entry is
  `{id, name, eventDate, templateId, items: []}`, reusing the existing
  `models.PackingList` type with `Items` left unpopulated. This mirrors the
  established `GET /templates` precedent (PACK-009: list mode always
  returns `items: []` by design; the grouped/detailed view is a separate
  endpoint's job).
- **`archived` query param**: only the literal string `"true"` selects the
  archived branch (`archived_at IS NOT NULL`). Anything else — absent,
  `"false"`, garbage — falls through to the active branch
  (`archived_at IS NULL`). No 400 for invalid values, consistent with how
  `category_id`/`search` are handled permissively on `GET /items`.
- **Ordering differs by branch**: active lists `ORDER BY updated_at DESC`
  (matches `GetTemplates`'s existing precedent — "recently touched" is the
  useful signal for a list you're still packing). Archived lists
  `ORDER BY archived_at DESC` — `updated_at` is frozen from before
  archiving, so `archived_at` is the temporal axis that actually matters
  for "what did I just finish." `archived_at` is usable in the `ORDER BY`
  even though it isn't returned in the JSON body (see below).
- **`archivedAt` is omitted from every response for now** (not `GET /lists`,
  not `GET /lists/:id`). Reconsidered mid-interview: there's no concrete
  consumer yet — the client already knows whether it's looking at an
  archived list from which endpoint/query it called, and DELETE's effect
  (moving between the two list views) is observable without the raw
  timestamp. Can be added later if a UI need shows up.
- **Archived lists remain fully fetchable via `GET /lists/:id`** — archiving
  only changes which list view (`GET /lists` vs `?archived=true`) a list
  shows up in, not whether its detail is reachable. Rationale: this is a
  travel-packing app, so "look back at what I packed for a past trip" is a
  plausible real use case, and soft-delete (vs hard delete) only makes
  sense if the detail survives archiving. Consistent with `ItemIsInUse`
  already treating archived lists as inactive for *write*/blocking
  purposes without affecting *read* access.
- **`GET /lists/:id` returns a new nested shape**, not the flat `items`
  array from `POST /lists`:
  - `models.PackingListDetail{ID, Name, EventDate, TemplateID, Categories, UserID (json:"-")}`
  - `models.PackingListCategory{ID, Name, Items}`
  - `models.PackingListDetailItem{ItemID, Name, Quantity, Notes, IsPacked}`
    — deliberately **no `categoryId`** here; it would just repeat the
    parent category's own `id` back to the client.
  - Categories ordered alphabetically by category name; items within each
    category ordered alphabetically by item name — matches the existing
    `ORDER BY items.name` precedent in `copyTemplateItemsTx`.
  - Only categories with ≥1 item **on this list** appear — no empty
    categories padded in.
  - `GetPackingListByID` (new repo method) always fetches and returns the
    full grouped detail, mirroring `GetTemplateByID`'s existing pattern of
    fetching items internally on every call. This same method backs the
    ownership check inside `Update`/`Delete`, exactly like
    `TemplateHandler.Update`/`Delete` reuse `GetTemplateByID` for their own
    ownership checks even though they don't need the items it also fetches.
- **`PATCH /lists/:id`** — update `name` and/or `eventDate`:
  - Body: `{name?, eventDate?}`, 400 if both omitted (mirrors
    Template/Category/Item `Update`'s "at least one of X" pattern).
  - `name`: same `validateName` as `POST /lists` (required non-empty,
    ≤100 chars, if present). **No uniqueness check** — matches PACK-010's
    decision that duplicate list names are fine.
  - `eventDate`: same `"2006-01-02"` parse, 400 if unparseable. Omitted
    means unchanged. **Explicitly clearing `eventDate` back to `null` is
    out of scope** — would need an omitted-vs-present-null distinction
    that doesn't exist anywhere in this codebase yet.
  - **Allowed on archived lists.** Archiving doesn't freeze the record —
    e.g. fixing a typo'd name on a past trip should still work — and
    there's no restore/unarchive endpoint in this ticket that would give a
    user another way to fix it.
  - 404 if the list doesn't exist or isn't owned by the caller.
  - Response is the **same full detail shape as `GET /lists/:id`**
    (grouped categories, re-fetched) — mirrors `UpdateTemplate` returning
    the full `Template` with items re-fetched rather than just the bare
    updated columns.
- **`DELETE /lists/:id`** — soft delete: sets `archived_at = CURRENT_TIMESTAMP`.
  404 if not found/not owned. **Idempotent** — deleting an already-archived
  list returns 204 again, no special-case check or error. 204, no body.
- **New `isPackingListOwned(list *models.PackingListDetail, userID string) bool`**
  in `ownership.go`, mirroring `isTemplateOwned` — packing lists have no
  system-level/nil-`UserID` concept (like templates, unlike
  categories/items), so it's a plain equality check.

## Acceptance criteria

- [ ] `GET /lists` — active lists for the caller.
  - [ ] 401 if unauthenticated.
  - [ ] Returns only lists with `archived_at IS NULL`, `ORDER BY updated_at DESC`.
  - [ ] Each entry: `id, name, eventDate, templateId, items: []`. No `archivedAt`.
- [ ] `GET /lists?archived=true` — archived lists for the caller.
  - [ ] Returns only lists with `archived_at IS NOT NULL`, `ORDER BY archived_at DESC`.
  - [ ] Any other `archived` value (missing, `"false"`, garbage) behaves
        identically to the default (active-only) branch.
- [ ] `GET /lists/:id` — detail view, items grouped by category.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID.
  - [ ] 404 if the list doesn't exist or isn't owned by the caller.
  - [ ] Works identically whether the list is archived or active.
  - [ ] Response: `id, name, eventDate, templateId, categories: [{id, name, items: [{itemId, name, quantity, notes, isPacked}]}]`.
  - [ ] Categories alphabetical by name; items within each category
        alphabetical by name; categories with zero items on this list are
        absent (not included as empty).
  - [ ] No `archivedAt` in the response.
- [ ] `PATCH /lists/:id` — update name and/or eventDate.
  - [ ] 401 if unauthenticated. 400 if neither `name` nor `eventDate` is present.
  - [ ] `name` (if present): 400 if empty/missing after trim or over 100 chars. No uniqueness check.
  - [ ] `eventDate` (if present): 400 if not a valid `YYYY-MM-DD` date.
  - [ ] 400 if `:id` isn't a valid UUID. 404 if not found/not owned.
  - [ ] Succeeds on an archived list (name/date edit not blocked by archiving).
  - [ ] Response is the same grouped-categories detail shape as `GET /lists/:id`, reflecting the update.
- [ ] `DELETE /lists/:id` — soft delete via `archived_at`.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if not found/not owned.
  - [ ] 204, sets `archived_at = CURRENT_TIMESTAMP`.
  - [ ] Idempotent: calling again on an already-archived list still returns 204 (no 409/error).

## Non-goals / files not touched by this ticket

- `POST /lists/:id/items`, `PATCH .../items/:itemId`, `DELETE .../items/:itemId`,
  `POST .../items/bulk` — PACK-012.
- `sort_order` assignment/reordering — PACK-012.
- `is_packed` toggling, `pack-all`/`unpack-all` — PACK-013.
- Any restore/"unarchive" endpoint — not in the backlog, not built here.
- Explicitly clearing `eventDate` to `null` via `PATCH`.
- Exposing `archivedAt` in any response.
- `POST /lists` (PACK-010) — its request/response shape is unchanged.
- Any change to `templates`/`items`/`categories` handlers, repos, or their tests.

## Expected test files

- `internal/repository/packing_list_test.go` (extend existing, package
  `repository_test`): `GetPackingLists` (active-only, archived-only,
  ordering for both branches); `GetPackingListByID` (found + owned,
  including an archived list still returning full detail; not-found →
  `nil, nil`); grouped categories/items shape, ordering, and empty-category
  exclusion; `UpdatePackingList` (name only, eventDate only, both, and on
  an already-archived list); `ArchivePackingList` (sets `archived_at`,
  idempotent re-archive).
- `internal/handler/packing_list_handler_test.go` (extend existing,
  package `handler_test`, `testify/mock` — extend
  `MockPackingListRepository` with `GetPackingLists`, `GetPackingListByID`,
  `UpdatePackingList`, `ArchivePackingList`): `List` (default active,
  `?archived=true`, invalid `archived` value falls back to active,
  unauthenticated); `GetByID` (owned, other-user's → 404, invalid id → 400,
  unauthenticated, archived list still 200); `Update` (name only, eventDate
  only, both, missing both → 400, invalid eventDate → 400, not owned → 404,
  invalid id → 400, unauthenticated, succeeds on an archived list); `Delete`
  (204, idempotent re-delete → 204, not owned → 404, invalid id → 400,
  unauthenticated).
- `requests/packing_lists.http` (extend existing): add `GET /lists`,
  `GET /lists?archived=true`, `GET /lists/:id` (active + archived),
  `PATCH /lists/:id`, `DELETE /lists/:id` sections. Also update the existing
  "Cleanup" section — its note that "there is no DELETE /lists/:id yet"
  becomes stale once this ticket ships; cleanup should actually complete
  (delete the seeded lists, then the item/category) instead of ending in
  the documented 409s.

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
