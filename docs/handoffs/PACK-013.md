# PACK-013 — Packing/ticking items

## Context

PACK-012 built adding/updating/removing/bulk-adding items on a packing
list. This ticket adds the "ticking" behavior the whole app exists for:
marking individual items packed/unpacked via the existing `PATCH
/lists/:id/items/:itemId`, plus two bulk convenience endpoints
(`pack-all`/`unpack-all`) for flipping every item on a list at once.

Key decisions from the interview:

- **`isPacked` extends the existing `PATCH /lists/:id/items/:itemId`
  (PACK-012)**, not a new endpoint — a 4th optional field alongside
  `quantity`/`notes`/`sortOrder`, combinable with any of them in the same
  request. `isPacked` is `*bool` (not `bool`) so `{"isPacked": false}` is
  distinguishable from omission, same reasoning as every other optional
  PATCH field in this codebase. The "at least one of ... is required" 400
  check extends to cover all four fields.
- **`POST /lists/:id/pack-all` and `POST /lists/:id/unpack-all` return
  `204 No Content`**, no body. Confirmed via the actual frontend
  consumption pattern: with an optimistic-update client (e.g. React
  Query), the mutation is fully deterministic and atomic (a single
  unconditional `UPDATE ... SET is_packed = <bool> WHERE list_id = $1`,
  not a loop over individual items) — there's no server-computed value or
  partial-failure state the client couldn't already predict from its own
  optimistic update, so a response body adds nothing. On failure, the
  client rolls back its own pre-mutation snapshot; the response body
  isn't part of that either.
- **Allowed on archived lists** — consistent with every mutation on this
  resource so far (PACK-011/012 all allow edits to archived lists); no
  reason to special-case packing state specifically.
- **A list with zero items is a harmless no-op** for `pack-all`/
  `unpack-all` — 204, not an error. There's no per-item existence check
  (unlike the single-item endpoints) since these target "every item
  currently on the list," not a specific `itemId`.
- **`UpdatePackingListItem` gains a 4th param `isPacked *bool`**,
  `COALESCE`'d like `quantity`/`notes`/`sortOrder`. This changes an
  existing repo method signature (again — same mechanical ripple as
  PACK-012's `itemRepo` constructor addition): every existing call site
  and mock (`internal/handler/packing_list_item_handler_test.go`,
  `internal/repository/packing_list_item_test.go`) needs updating to pass
  the new argument, even in tests that don't touch `isPacked` at all.
  Mechanical, not scope creep — flagging per the established convention.
- **New repository methods** `PackAllItems(ctx, listID string) error` and
  `UnpackAllItems(ctx, listID string) error` on the existing
  `PackingListRepository` — each a single unconditional `UPDATE`, no
  per-item loop, no existence checks.
- **New handler methods** `PackAll`/`UnpackAll` on the existing
  `PackingListHandler` (new file `internal/handler/packing_list_pack_handler.go`
  to avoid further bloating `packing_list_item_handler.go`), reusing the
  existing `requireOwnedPackingList` helper.
- **No new "isFullyPacked" summary field** on the list resource itself
  (`GET /lists` or `GET /lists/:id`) — not asked for by the backlog, and
  not needed for `pack-all`/`unpack-all` to work. Explicitly out of scope;
  a client can already derive this by checking each item's `isPacked`.

## Acceptance criteria

- [ ] `PATCH /lists/:id/items/:itemId` — now also accepts `isPacked`.
  - [ ] `isPacked` (if present): `true` or `false`, both valid; combinable
        with `quantity`/`notes`/`sortOrder` in the same request.
  - [ ] 400 if none of `quantity`/`notes`/`sortOrder`/`isPacked` are
        present (existing check extended, not replaced).
  - [ ] All existing PACK-012 acceptance criteria for this endpoint
        (ownership, 404s, 400s, archived-list success) continue to hold
        unchanged.
  - [ ] Response (single flat item, existing shape) reflects the updated
        `isPacked` value.
- [ ] `POST /lists/:id/pack-all` — mark every item on the list packed.
  - [ ] 401 if unauthenticated. 400 if `:id` isn't a valid UUID. 404 if
        the list doesn't exist or isn't owned by the caller.
  - [ ] 204, no body. Every `packing_list_items` row for this list has
        `is_packed = true` afterward, regardless of prior state.
  - [ ] A list with zero items: 204, no error.
  - [ ] Succeeds on an archived list.
- [ ] `POST /lists/:id/unpack-all` — reset every item on the list to
      unpacked.
  - [ ] Same shape as `pack-all`: 401/400/404 cases, 204 no body, sets
        every item's `is_packed = false`, zero-item no-op, succeeds on
        archived lists.

## Non-goals / files not touched by this ticket

- Any "isFullyPacked"/packed-count summary field on `GET /lists` or
  `GET /lists/:id`.
- Any reordering, quantity, or notes behavior — already shipped in
  PACK-012, untouched here except for the mechanical signature ripple.
- Any change to `POST /lists/:id/items`, `DELETE /lists/:id/items/:itemId`,
  or `POST /lists/:id/items/bulk`'s own logic (their signatures/mocks are
  unaffected — only `UpdatePackingListItem` gains a param).
- Any change to `templates`/`items`/`categories` handlers, repos, or their
  tests.

## Expected test files

- `internal/repository/packing_list_item_test.go` (extend existing):
  update every existing `UpdatePackingListItem` call site for the new
  `isPacked *bool` param (pass `nil` where the test doesn't care about
  it); add `TestUpdatePackingListItem_IsPackedOnly` (true and false
  cases) and a combination test with quantity/notes/sortOrder/isPacked
  together; add `TestPackAllItems` (sets every item's `is_packed = true`,
  including previously-true and previously-false ones) and
  `TestUnpackAllItems` (mirrors it), plus a zero-item no-op case for each.
- `internal/handler/packing_list_item_handler_test.go` (extend existing):
  update every existing `UpdatePackingListItem` mock expectation for the
  new `isPacked` argument; add `TestPackingListItemUpdate_IsPackedOnly`
  (true and false) and a combined-fields case.
- `internal/handler/packing_list_pack_handler_test.go` (new, mirrors the
  existing packing-list-item handler test style, uses `doRequest`):
  `PackAll`/`UnpackAll` — success (204), list not found, list not owned,
  invalid list id, unauthenticated, succeeds on an archived list. (Note:
  zero-item-list is a repository-level behavior: the same mocked
  "success" path from the handler's point of view, since the handler
  doesn't know or care how many items were affected — no separate handler
  test needed for it, per the "test should trace to something distinct"
  rule from PACK-012's retro.)
- `requests/packing_lists.http` (extend existing): add `isPacked` cases to
  the existing self-contained `PATCH /lists/:id/items/:itemId` section (or
  a new self-contained section if that one has already drifted since
  PACK-012 — check at implementation time), and new self-contained
  `POST /lists/:id/pack-all` / `POST /lists/:id/unpack-all` sections, each
  drafted alongside its own handler implementation with an explicit
  manual-check gate per commit.

## Close-out

Completed 2026-07-10. Retro entry in LESSONS.md.
