# PACK-036 — ItemCount/PackedCount on GetPackingLists

## Context

Cross-repo gap flagged during `packing-list-react`'s PACKFE-005 grill-me
(2026-07-27, confirmed by reading current source, not assumed) and raised
again now that its Piece 7 (list/rail assembly) actually needs it: `GET
/lists` (list mode, `GetPackingLists`) returns every list with `Items`
always empty (confirmed, `internal/repository/packing_list.go` — matches
`GetTemplates`' pre-fix behavior exactly) and has no per-list count at
all. The trips list/rail UI needs both a total item count and a packed
count per trip (`"4 of 13 packed"`), the same shape `GetTemplates` needed
before its own `ItemCount` fix
(`internal/models/template.go`/`internal/repository/template.go`,
shipped).

This is a narrow, fully-precedented mirror of that existing fix, not new
design — no interview needed, the shape is already settled:

- **Two new fields on `models.PackingList`**: `ItemCount int
json:"itemCount"` and `PackedCount int json:"packedCount"`.
  `PackingListDetail` is untouched — its `Categories[].Items[]` already
  carries every item's real `IsPacked`, so a client can already derive
  both counts for the *detail* view itself; these two fields only matter
  for *list* mode, same division `GetTemplates`' `ItemCount` already
  established for templates.
- **`GetPackingLists` gets its own inline scan, not `scanPackingList`**
  — mirrors `GetTemplates`' own comment
  (`internal/repository/template.go:22-24`) for the identical reason: no
  other caller's query selects these two columns, and a `LEFT JOIN`/
  `GROUP BY` would drag `ORDER BY updated_at` into the group-by list.
  `scanPackingList` (shared with `GetPackingListByID`) is untouched.
  Two correlated `COUNT` subqueries: `(SELECT COUNT(*) FROM
packing_list_items WHERE list_id = packing_lists.id)` for
  `ItemCount`, same shape again with `AND is_packed = true` for
  `PackedCount`.
- **No handler changes.** `PackingListHandler.List`
  (`internal/handler/packing_list_handler.go:104-120`) is pure
  pass-through (`c.JSON(http.StatusOK, lists)`) — the new fields flow
  through automatically once the repository populates them, exactly as
  `TemplateHandler.List` needed no changes for its own `ItemCount`.

## Acceptance criteria

- [x] `models.PackingList` gains `ItemCount int json:"itemCount"` and
      `PackedCount int json:"packedCount"`
- [x] `GetPackingLists` returns the real total item count and real packed
      count per list, via its own inline scan (not `scanPackingList`)
- [x] A list with zero items returns `itemCount: 0, packedCount: 0`, not
      an error or null
- [x] `GetPackingListByID`/`PackingListDetail` are unaffected — no new
      fields, no behavior change (counts are trivially derivable
      client-side from `Categories[].Items[]` there already)
- [x] `GET /lists` response includes both fields per list (handler needed
      no code change — verified via a handler test, same as
      `TestTemplateList_IncludesItemCount`)

## Non-goals

- No change to `GetPackingListByID`/detail-mode response shape
- No change to the `PackingListItem`/`PackingListDetailItem` models
- No OpenAPI spec update (PACK-026, separate ticket, not started)
- No batching/N+1 concern — same single-query-with-subqueries shape
  already accepted for `GetTemplates`, same list sizes

## Expected test files

- `internal/repository/packing_list_test.go` — new
  `TestGetPackingLists_ItemCountAndPackedCount` (integration, real Neon
  dev DB, mirrors `TestGetTemplates_ItemCount`'s style): seed a list with
  two items via `AddPackingListItem` (reusing the existing
  `createTestCategory`/`createTestItem`/`createTestTemplate`-style
  helpers already in this file), mark one packed via
  `UpdatePackingListItem`, assert `ItemCount == 2, PackedCount == 1` in
  the `GetPackingLists` results; a second list with zero items in the
  same test asserts `ItemCount == 0, PackedCount == 0`.
- `internal/handler/packing_list_handler_test.go` — new
  `TestPackingListList_IncludesItemCountAndPackedCount`, mirroring
  `TestTemplateList_IncludesItemCount`'s mocked-repo style exactly (set
  `ItemCount`/`PackedCount` on a `packingListResult(...)` fixture, assert
  the JSON response's `itemCount`/`packedCount` fields).
- No `.http` request file change — `GET /lists` is already covered there;
  the new fields just show up in the existing response, no new manual
  case needed.

## Close-out

Completed 2026-07-29. Small, fully-precedented mirror of `GetTemplates`'
`ItemCount` fix — no new lessons, no LESSONS.md entry warranted on its
own.
