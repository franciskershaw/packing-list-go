# PACK-009 — Managing items on a template

## Context

PACK-008 built template CRUD but left `Template.Items` hard-coded to `[]`
in every response — there was no way to attach items yet, and the real
`template_items` join was explicitly deferred here. This ticket adds that:
attaching/updating/removing items on a template, bulk-adding a whole
category's worth at once, and (critically) making those items actually
visible through `GET /templates/:id`, which nothing else in the backlog
covers (PACK-011's similarly-worded "list with items grouped by category"
is for packing lists, a different resource entirely).

Key decisions from the interview:

- **One row per `(template, item)`.** `quantity` is how "5 pairs of pants"
  is expressed — not five rows. `POST` for an item already on the template
  is a 409; to change how many, use `PATCH .../items/:itemId`.
- **Real items join, detail endpoint only.** `GetTemplateByID` gets a
  second, separate query for that one template's items (not a SQL `JOIN`
  on the existing `SELECT`) — avoids the row-fanout bug a naive `JOIN` +
  flat scan-loop would cause (a template with N items silently returning N
  duplicate `Template` objects). `GetTemplates` (list) keeps returning
  `items: []`; the spec only ever promised items on the detail view.
- **Mutation endpoints return the single `TemplateItem` affected**, not the
  whole `Template` — matches the existing `POST/PATCH /items` precedent
  (returns the `Item`, not its parent `Category`), and is the friendlier
  shape for a client to patch its own cache with. Bulk-add returns an array
  of the newly-added items.
- **Bulk-add skips items already on the template** rather than erroring —
  it's a convenience "add everything from this category I don't already
  have" action; 201 with whatever was newly added (possibly `[]` if
  nothing was new). Cheap to implement (one extra query, a set-difference)
  and robust regardless of whether the future UI already filters before
  calling it.
- **`TemplateHandler` gets a second repository dependency**: a small,
  handler-defined interface exposing just the `ItemRepository` methods it
  needs (`GetItemByID`, `GetItems`, `CategoryIsAccessible`) — reused rather
  than re-implementing item lookup/accessibility queries inside
  `TemplateRepository`, which would just duplicate `item.go`'s SQL. First
  two-repo handler in the codebase; nothing in the architecture rules
  forbids it (they describe consumer-defined interfaces per handler, not
  "one repository per handler").
- **Quantity**: integer, `>= 1` and `<= 999`, 400 otherwise. High enough to
  cover realistic bulk cases (diapers, snacks, meds for a long trip)
  without second-guessing a legitimate large quantity — the cap exists
  only to catch typos/absurd input (e.g. an extra zero or two), not to
  second-guess plausible packing scenarios.
- **Notes**: optional, trimmed, `<= 200` chars, 400 otherwise. Shorter cap
  than template `description` (500) — a per-item note is realistically a
  short aside, not a paragraph.
- **PATCH semantics** mirror PACK-008's template PATCH exactly: at least
  one of `quantity`/`notes` required (400 otherwise), omitted fields mean
  "leave unchanged," `notes: ""` clears to an empty string (not `NULL`).
- **New files**: `internal/handler/template_item_handler.go` and
  `internal/repository/template_item.go` — same `TemplateHandler` /
  `TemplateRepository` structs (a Go type's methods can span files in the
  same package), split out so `template_handler.go`/`template.go` don't
  grow to cover 9 endpoints' worth of code.
- Ownership: template must exist and be owned by the caller (404
  otherwise) before any item-on-template operation, per the existing
  `isTemplateOwned` check.
- `itemId` on `POST` is a body field (the item isn't addressable on this
  path until attached) — 400 for missing/invalid UUID/inaccessible,
  matching `validateAccessibleCategory`'s pattern for `categoryId` on item
  creation. `itemId` on `PATCH`/`DELETE` is a path param — 400 for
  malformed UUID, 404 if that item isn't currently on the template.
- `template_items.item_id` has `ON DELETE CASCADE`, and item deletion is
  already blocked (409, `ItemIsInUse`) while referenced by a template — so
  a `template_items` row can never outlive its item in normal operation.

## Acceptance criteria

- [ ] `POST /templates/:id/items` — attach an item to the template.
  - [ ] 404 if the template doesn't exist or isn't owned by the caller.
  - [ ] `itemId` required in body; 400 if missing, invalid UUID, or not
        accessible to the caller (system or their own item).
  - [ ] `quantity` optional, defaults to 1; 400 if present and not an
        integer in `[1, 999]`.
  - [ ] `notes` optional, trimmed, 400 if over 200 chars.
  - [ ] 409 if this item is already on the template.
  - [ ] 201 with the created `TemplateItem` (`{itemId, name, quantity,
        notes}`) on success.
- [ ] `PATCH /templates/:id/items/:itemId` — update quantity/notes.
  - [ ] 404 if the template doesn't exist/isn't owned, or if this item
        isn't currently on the template.
  - [ ] 400 if neither `quantity` nor `notes` is present in the body.
  - [ ] Same quantity/notes validation as `POST`.
  - [ ] 200 with the updated `TemplateItem` on success.
- [ ] `DELETE /templates/:id/items/:itemId` — remove an item from the
      template.
  - [ ] 404 if the template doesn't exist/isn't owned, or if this item
        isn't currently on the template.
  - [ ] 204 on success.
- [ ] `POST /templates/:id/items/bulk` — add every accessible item from a
      given category.
  - [ ] 404 if the template doesn't exist/isn't owned.
  - [ ] `categoryId` required in body; 400 if missing, invalid UUID, or not
        accessible to the caller.
  - [ ] Items in that category already on the template are skipped, not
        errored.
  - [ ] 201 with an array of the newly-added `TemplateItem`s (`[]` if
        everything in that category was already present, or the category
        has no items).
- [ ] `GET /templates/:id` now returns real items (was hard-coded `[]`
      since PACK-008) via a second query, not a SQL join on the existing
      `GetTemplateByID` `SELECT`.
  - [ ] A template with multiple items returns exactly one `Template`
        object with all its items in the `items` array (no row-fanout
        duplication).
- [ ] `GET /templates` (list) is unchanged — still returns `items: []` per
      template.

## Non-goals / files not touched by this ticket

- Populating items in the `GET /templates` list view — detail endpoint
  only (see decision above).
- Any change to `categories`/`items` handlers, repos, or their tests
  (`ItemRepository`'s existing methods are reused, not modified).
- Packing lists / `packing_list_items` (Epic 5, PACK-010 onward) — a
  separate resource, not covered by this ticket despite similar wording in
  the PACK-011 spec entry.
- Reordering or sorting template items — no `sort_order` concept exists
  for `template_items` (unlike `packing_list_items`, which has one).

## Expected test files

- `internal/repository/template_item_test.go` — real-DB integration tests
  (package `repository_test`): `AddTemplateItem`, `UpdateTemplateItem`
  (quantity only, notes only, both, empty-string-notes-not-null),
  `RemoveTemplateItem`, `TemplateItemExists`, `GetTemplateItems` (multiple
  items, empty template), and confirming `GetTemplateByID` returns the
  right items with no duplication for a template with 2+ items.
- `internal/handler/template_item_handler_test.go` — mocked-repo handler
  tests (package `handler_test`, `testify/mock`, two mocks: template repo
  + the narrow item-repo interface): add (success, missing/invalid/
  inaccessible itemId, quantity out of range, notes too long, duplicate
  409, template not found), update (success on quantity, success on
  notes, 400 when both omitted, not found on template or item, validation
  reused from add), remove (success, not found on template or item),
  bulk (success with some skipped, success with none skipped, empty
  category, invalid/inaccessible categoryId, template not found).
- `requests/template_items.http` — new manual verification file (not an
  extension of `requests/templates.http`), including its own template +
  item + category setup section, mirroring how `requests/items.http`
  duplicates category setup rather than depending on
  `requests/categories.http`.
