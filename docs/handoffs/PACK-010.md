# PACK-010 — Packing list creation

## Context

PACK-008/PACK-009 built templates and the ability to attach items to them.
This ticket introduces the first packing-list-facing endpoint: creating a
packing list, optionally seeded from a template's items. Everything past
creation — reading, updating, archiving lists (PACK-011), managing items on
an already-created list (PACK-012), and packing/ticking (PACK-013) — is
explicitly out of scope here.

Key decisions from the interview:

- **The `201` response includes the copied items**, as a flat array — not
  grouped by category (that's PACK-011's `GET /lists/:id` concern). Unlike
  PACK-008's template creation (which genuinely started empty), a
  `template_id`-seeded list already has real `packing_list_items` rows by
  the time this endpoint returns; responding with `items: []` would be
  actively misleading, not just incomplete.
- **`sort_order` is left `NULL`** for every copied item — no alphabetical or
  template-order default is assigned. Ordering is PACK-012's concern
  (`PATCH .../items/:itemId` there explicitly covers "update
  quantity/notes/sort order," per the backlog). Confirmed this will
  definitely be picked up later, not dropped.
- **`archivedAt` and `sortOrder` are omitted from the JSON response
  entirely** for now, following the precedent in `template.go` of only
  exposing columns relevant to shipped functionality (e.g. `created_at`/
  `updated_at` aren't in the `Template` JSON either). Both columns exist in
  the schema already; they're populated/exposed by later tickets
  (`archivedAt` in PACK-011, `sortOrder` in PACK-012).
- **`name`**: required, trimmed, ≤100 chars — reuses the existing
  `validateName` helper. **No uniqueness check** — unlike templates,
  duplicate list names are fine (e.g. "Weekend Trip" in different years).
- **`eventDate`**: optional `*string` in `"YYYY-MM-DD"` format (matches the
  `DATE` column), 400 if present but not parseable via the `"2006-01-02"`
  layout. No past/future restriction.
- **`templateId`**: optional. If present: 400 if not a valid UUID, and 400
  (not 404) if it doesn't belong to the caller — consistent with how
  `categoryId`/`itemId` are validated as body-referenced foreign keys
  elsewhere in this codebase (400 is reserved for "your input is
  unusable"; 404 is reserved for the resource the URL itself addresses,
  and `POST /lists` has no path param to 404 against). Reuses the existing
  `isTemplateOwned` helper for the ownership check (templates have no
  system-level concept, so "not owned" and "doesn't exist" both collapse
  to the same 400).
- **Ownership check happens handler-side**, via a new small
  `TemplateLookupRepository` interface (`GetItemByID`-style pattern from
  PACK-009's `ItemLookupRepository`) exposing just `GetTemplateByID` —
  consistent with every other foreign-key check in this codebase happening
  handler-side before the mutating repository call, even though that means
  the check isn't inside the same transaction as the insert. (The
  check-then-create race this implies already exists identically for
  `categoryId`/`itemId` elsewhere and hasn't been treated as a concern.)
  `handler.MockTemplateRepository` already implements `GetTemplateByID`, so
  it can be reused directly as the test double — no new mock type needed.
- **List creation + item copy is one DB transaction** inside
  `PackingListRepository.CreatePackingList` — if the copy fails partway
  through, no orphaned list is left behind. The copy itself is the
  repository's own SQL (`template_items JOIN items` for `category_id`),
  not a call back into `TemplateRepository` — mirrors PACK-009's decision
  to keep this kind of data-shaping query inside the owning repository.
- **New files**: `internal/models/packing_list.go` (`PackingList`,
  `PackingListItem`), `internal/repository/packing_list.go`
  (`PackingListRepository`), `internal/handler/packing_list_handler.go`
  (`PackingListHandler`). `internal/repository/main_test.go` (shared
  `TestMain` setup) needs a `packingListRepo` variable added — this is an
  edit to an existing shared file, not purely new.
- DB driver is `lib/pq` (`database/sql`); `event_date` scans via
  `sql.NullTime`, formatted to `"2006-01-02"` for the JSON string.

## Acceptance criteria

- [ ] `POST /lists` — create a packing list.
  - [ ] 401 if unauthenticated.
  - [ ] `name` required, trimmed, 400 if missing/empty or over 100 chars.
  - [ ] `eventDate` optional; 400 if present and not a valid `YYYY-MM-DD`
        date.
  - [ ] `templateId` optional; 400 if present and not a valid UUID, or not
        owned by the caller.
  - [ ] Without `templateId`: 201, `items: []`.
  - [ ] With `templateId`: 201, and every row from that template's
        `template_items` is copied into `packing_list_items` — `item_id`,
        `quantity`, and `notes` preserved verbatim; `category_id` populated
        from `items.category_id`; `is_packed` defaults `false`;
        `sort_order` left `NULL`. Response `items` array reflects the copy
        (`itemId`, `name`, `categoryId`, `quantity`, `notes`, `isPacked`).
  - [ ] With a `templateId` that has zero items: 201, `items: []`, no
        error.
  - [ ] List creation and item copy happen atomically (single transaction).
  - [ ] Response never includes `archivedAt` or `sortOrder`.

## Non-goals / files not touched by this ticket

- `GET /lists`, `GET /lists/:id`, `PATCH /lists/:id`, `DELETE /lists/:id`
  (soft delete via `archived_at`) — all PACK-011.
- The grouped-by-category detail view — PACK-011's `GET /lists/:id`, not
  this endpoint's flat `items` array.
- Adding/updating/removing items on an already-created list, and bulk-add
  from a category — PACK-012.
- Assigning or reordering `sort_order` — PACK-012.
- Packing/ticking items, `pack-all`/`unpack-all` — PACK-013.
- Any change to `templates`/`items`/`categories` handlers, repos, or their
  tests.

## Expected test files

- `internal/repository/packing_list_test.go` — real-DB integration tests
  (package `repository_test`): `CreatePackingList` with name only, with
  `eventDate`, with a `templateId` whose items get copied correctly
  (including `category_id`/`quantity`/`notes` fidelity and `sort_order`
  staying `NULL`), and with a `templateId` pointing at an empty template.
- `internal/handler/packing_list_handler_test.go` — mocked-repo handler
  tests (package `handler_test`, `testify/mock`; reuses the existing
  `MockTemplateRepository` for the `TemplateLookupRepository` dependency):
  Create (valid without template, valid with `eventDate`, valid with
  `templateId`, missing/empty/too-long name, invalid `eventDate` format,
  missing/invalid/other-user's `templateId`, unauthorized).
- `internal/repository/main_test.go` — add `packingListRepo` to the shared
  `TestMain` setup (existing file, edited not created).
- `requests/packing_lists.http` — new manual verification file, with its
  own template + item + category setup section (mirroring how
  `template_items.http` duplicates setup rather than depending on other
  `.http` files).

## Close-out

Completed 2026-07-09. Retro entry in LESSONS.md.
