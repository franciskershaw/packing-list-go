# PACK-012 — Managing items on a packing list

## Context

PACK-010 created lists (optionally seeded from a template); PACK-011 added
listing/detail/rename/archive. This ticket is the packing-list equivalent
of PACK-009 (managing items on a template) — add/update/remove a single
item, plus bulk-add every item from a category. It's also the first ticket
to actually populate and expose `sort_order`, which both PACK-010's and
PACK-011's handoff docs explicitly deferred to here. Packing/ticking
(`is_packed`, pack-all/unpack-all) remains PACK-013.

Key decisions from the interview:

- **Mirrors PACK-009's template-item endpoints closely** — same four
  endpoints, same ownership-helper shape (`requireOwnedPackingList`,
  analogous to `TemplateHandler.requireOwnedTemplate`), same "add already
  on it → 409" / "bulk-add skips duplicates silently" semantics, same
  `validateQuantity` ([1, 999]) and `validateTemplateItemNotes` (≤200
  chars, reused as-is from `template_item_handler.go` — its name says
  "template" even though it's now also called from list-item code;
  renaming it isn't done in this ticket since it'd touch PACK-009's file
  for a cosmetic reason — flagged for the tech-debt epic instead, see
  `docs/handoffs/PACK-014.md`).
- **`categoryId` is never in the request body** — derived server-side from
  the item's own `category_id`, matching how `CreatePackingList`'s
  template-copy already populates `packing_list_items.category_id`. Item
  accessibility is system-or-owned (`isItemAccessible`), not
  strictly-owned — matches PACK-009, not the stricter `isItemOwned`.
- **Item mutations are allowed on archived lists** — consistent with
  PACK-011's decision that archiving doesn't freeze the record, and there's
  no restore/unarchive endpoint that would give another way to fix an
  archived list's contents.
- **Adding an item already on the list → 409 conflict**, matching
  `TemplateItemExists` → 409 exactly.
- **`sort_order` semantics**:
  - Not settable at creation (`POST /lists/:id/items`) — new items always
    start with `sort_order NULL`, matching PACK-010's template-copy
    behavior. Only `PATCH .../items/:itemId` can set it.
  - Any integer is valid, including negative or zero — no domain
    constraint like `quantity`'s range, since it's purely a
    client-managed relative-ordering key with no `CHECK` constraint on the
    column. No dedicated validation helper needed.
  - `PATCH` accepts `quantity?`, `notes?`, `sortOrder?` — at least one
    required (matches the existing "at least one of X" pattern). Omitted
    means unchanged for each field independently; there's no way to
    explicitly clear `sortOrder` back to `NULL` in this ticket (same
    omitted-vs-null gap already accepted for `eventDate` in PACK-011).
  - **`GET /lists/:id` now orders items within each category by
    `sort_order` (NULLS LAST), falling back to alphabetical by name** —
    both for items sharing a `sort_order` value and for the
    still-`NULL`/never-touched ones. This changes `getPackingListCategories`'s
    query (previously pure `ORDER BY c.name, i.name`) to
    `ORDER BY c.name, pli.sort_order NULLS LAST, i.name`. A list is never
    in an arbitrary order: explicitly-ordered items surface first, in that
    order, and everything else still reads alphabetically.
- **Response shapes for the single-item endpoints (`POST`, `PATCH`) return
  just the one affected item, flat** — `{itemId, name, categoryId,
  quantity, notes, isPacked, sortOrder}` — matching PACK-009's
  `AddTemplateItem`/`UpdateTemplateItem` precedent (they return just the
  one `TemplateItem`, not the whole template). This is a deliberate
  departure from PACK-011's `PATCH`/`DELETE /lists/:id` precedent, which
  re-fetches and returns the full grouped detail — different because those
  endpoints mutate the list itself, while these mutate one item on it.
  - `models.PackingListItem` gains a new `SortOrder *int
    json:"sortOrder"` field (previously undefined — PACK-010 explicitly
    left it unexposed). This also means `POST /lists`'s existing response
    (PACK-010) now additionally carries `sortOrder: null` for every
    copied item, since it shares the same struct — not a breaking change,
    just a new always-`null`-until-now field appearing.
  - `models.PackingListDetailItem` (the nested-under-category shape from
    PACK-011's `GET /lists/:id`) also gains `SortOrder *int
    json:"sortOrder"`.
- **`POST /lists/:id/items/bulk`** — adds every item from a given category,
  silently skipping ones already on the list (matches PACK-009's
  `BulkAddItems` exactly: fetch existing list items once, build a set,
  skip hits). Every bulk-added item defaults `quantity: 1, notes: null,
  sortOrder: null` — no per-item overrides in the bulk call itself,
  matching the backlog's plain "add every item from a category" wording.
  Response is a flat array of just the newly-added items (201), empty
  array if everything in the category was already present.
- **New repository methods** on the existing `PackingListRepository`:
  `AddPackingListItem`, `UpdatePackingListItem`, `RemovePackingListItem`,
  `PackingListItemExists`, `GetPackingListItems` (flat, for the bulk-add
  duplicate check — mirrors `GetTemplateItems`'s role for
  `BulkAddItems`'s `alreadyOnTemplate` map).
- **New handler file** `internal/handler/packing_list_item_handler.go`
  (mirrors `template_item_handler.go`'s split from `template_handler.go`)
  holding `AddItem`/`UpdateItem`/`RemoveItem`/`BulkAddItems` on the
  existing `PackingListHandler`, plus its own `requireOwnedPackingList`
  helper (mirrors `requireOwnedTemplate`).
- **`PackingListHandler` gains a new constructor dependency**: `itemRepo
  ItemLookupRepository` (the same interface already defined in
  `template_handler.go` — `GetItemByID`, `GetItems`, `CategoryIsAccessible`
  — reused directly, not redefined). This is a breaking change to
  `NewPackingListHandler`'s signature, which means:
  - `main.go`'s existing `packingListHandler := handler.NewPackingListHandler(...)`
    call needs the new argument.
  - `internal/handler/packing_list_handler_test.go`'s existing
    `newPackingListTestRouter` helper, and every existing PACK-010/011 test
    that calls it, need updating to pass an `itemRepo` mock too. This is a
    mechanical, non-behavioral change forced by extending the same handler
    struct those tickets already built — not scope creep into their logic.
    `MockItemRepository` (already defined in `item_handler_test.go`, same
    `handler_test` package) is reused directly — no new mock type needed.

## Acceptance criteria

- [ ] `POST /lists/:id/items` — add an item to a list.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if the
        list doesn't exist or isn't owned by the caller.
  - [ ] Body: `itemId` (required, valid UUID, 400 otherwise; 400 if not
        accessible — system-owned or the caller's own), `quantity`
        (optional, default 1, 400 if outside [1, 999]), `notes` (optional,
        400 if over 200 chars after trim).
  - [ ] 409 if the item is already on this list.
  - [ ] 201: `categoryId` populated from the item's own category;
        `isPacked: false`; `sortOrder: null`. Response is just the added
        item, flat.
  - [ ] Succeeds on an archived list.
- [ ] `PATCH /lists/:id/items/:itemId` — update quantity/notes/sortOrder.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if the
        list doesn't exist/isn't owned, or the item isn't on this list.
  - [ ] 400 if `itemId` isn't a valid UUID. 400 if none of
        `quantity`/`notes`/`sortOrder` are present.
  - [ ] `quantity` (if present): 400 if outside [1, 999].
  - [ ] `notes` (if present): 400 if over 200 chars after trim.
  - [ ] `sortOrder` (if present): any integer accepted, including negative
        or zero.
  - [ ] 200: the single updated item, flat, reflecting only the fields
        that were present in the request.
  - [ ] Succeeds on an archived list.
- [ ] `DELETE /lists/:id/items/:itemId` — remove an item from a list.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if the
        list doesn't exist/isn't owned, or the item isn't on this list.
  - [ ] 204 on success. Succeeds on an archived list.
- [ ] `POST /lists/:id/items/bulk` — add every item from a category.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if the
        list doesn't exist or isn't owned by the caller.
  - [ ] Body: `categoryId` required; 400 if missing/invalid UUID, 400 if
        not accessible.
  - [ ] Items already on the list are silently skipped, not errored.
  - [ ] 201: flat array of only the newly-added items, each with
        `quantity: 1, notes: null, sortOrder: null`. Empty array (not an
        error) if every item in the category was already on the list, or
        the category has no items.
  - [ ] Succeeds on an archived list.
- [ ] `GET /lists/:id` (PACK-011, being extended) — items within each
      category now order by `sort_order` (NULLS LAST) then item name,
      instead of purely alphabetically. Nested items now include
      `sortOrder`.
- [ ] `POST /lists` (PACK-010, being extended) — response items now
      include `sortOrder` (always `null`, since template-copied items
      still get `sort_order NULL`).

## Non-goals / files not touched by this ticket

- `is_packed` toggling, `pack-all`/`unpack-all` — PACK-013.
- Any reordering algorithm/endpoint (e.g. "move item to position X,
  shift the rest") — `sortOrder` is a plain client-managed integer field,
  set directly via `PATCH`, nothing more.
- Explicitly clearing `sortOrder` back to `NULL` via `PATCH`.
- Renaming `validateTemplateItemNotes` or otherwise touching
  `template_item_handler.go`/`template_item.go` — flagged for the
  tech-debt epic instead.
- Any change to `templates`/`items`/`categories` handlers, repos, or their
  tests, beyond reusing `MockItemRepository` as a test double.
- `PATCH`/`DELETE /lists/:id` (list-level, PACK-011) — unchanged except for
  the ordering fix inside `GET /lists/:id`'s existing query.

## Expected test files

- `internal/repository/packing_list_item_test.go` (new, package
  `repository_test`, mirrors `template_item_test.go`): `AddPackingListItem`
  (default quantity, with notes, populates `categoryId` from the item),
  `UpdatePackingListItem` (quantity only, notes only, sortOrder only,
  combinations, negative/zero sortOrder), `RemovePackingListItem`,
  `PackingListItemExists` (true/false), `GetPackingListItems` (flat, for
  the bulk-add dup check). Also extend
  `internal/repository/packing_list_test.go`'s
  `TestGetPackingListByID_GroupedByCategoryAlphabetical`-style coverage (or
  add a new test) confirming `sort_order` now takes precedence over
  alphabetical ordering, with a NULLS-LAST/alphabetical-fallback case
  mixed in.
- `internal/handler/packing_list_item_handler_test.go` (new, package
  `handler_test`, `testify/mock`, uses the shared `doRequest` helper per
  the PACK-011 retro convention): `AddItem` (valid default quantity, valid
  with quantity/notes, invalid/missing itemId, inaccessible item,
  duplicate → 409, list not found/not owned, invalid list id,
  unauthenticated, succeeds on archived list); `UpdateItem` (quantity only,
  notes only, sortOrder only including negative, missing-all-three → 400,
  item not on list → 404, invalid ids, unauthenticated, succeeds on
  archived list); `RemoveItem` (204, not found variants, invalid ids,
  unauthenticated, succeeds on archived list); `BulkAddItems` (adds new,
  skips existing, empty category, invalid/inaccessible categoryId, list
  not found/not owned, unauthenticated, succeeds on archived list).
- `internal/handler/packing_list_handler_test.go` (existing file, edited
  not created): update `newPackingListTestRouter` and every existing
  Create/List/GetByID/Update/Delete test's call site for the new
  `itemRepo` constructor argument (`MockItemRepository`, reused as-is).
- `requests/packing_lists.http` (extend existing): add `POST
  /lists/:id/items`, `PATCH .../items/:itemId`, `DELETE .../items/:itemId`,
  `POST .../items/bulk` sections, each drafted alongside its own handler's
  implementation with an explicit manual-check gate per commit — per the
  PACK-011 retro convention, not all upfront.
