# PACK-029 — Unarchive endpoint

Add a way to reverse `DELETE /lists/:id`'s soft-archive, so a list can be
restored to active without a fresh OAuth/recreate round-trip.

## Context

`DELETE /lists/:id` sets `archived_at` but nothing currently reverses it
(`internal/repository/packing_list.go:255`, `internal/handler/packing_list_handler.go:216`).
Found 2026-07-11 during frontend prototyping: the prototype's archive
toggle is designed as reversible (tap again to restore), which the API
can't currently support. Filed in `LESSONS.md` as priority — pick up
before PACK-021 (unrelated server-lifecycle ticket, still not started, so
that ordering still holds).

**Design gate**: follows precedent, no new pattern, no ADR needed (project
has no `docs/adr/` yet). Key decisions from the interview:

- **Route**: `POST /lists/:id/unarchive`, not a `PATCH /lists/:id` body
  field. Mirrors the `PackAll`/`UnpackAll` precedent
  (`internal/handler/packing_list_pack_handler.go`) — a reversible pair
  gets two action-verb `POST` routes, not one endpoint with a boolean.
- **Repo method**: `UnarchivePackingList(ctx, id) error` — unconditional
  `SET archived_at = NULL WHERE id = $1`, idempotent no-op if already
  active, mirroring `ArchivePackingList`'s own documented behavior exactly.
- **Ownership check**: use the `requireOwnedPackingList` helper
  (`internal/handler/packing_list_item_handler.go:16`), not the older
  inline `GetPackingListByID` + `isPackingListOwned` pattern that `Delete`
  predates it with. `PackAll`/`UnpackAll` are the closest structural
  precedent and already use the helper.
- **Response**: `204 No Content`, no body — matches `Delete` (its direct
  inverse) and `PackAll`/`UnpackAll` (closest structural precedent).
- **File placement**: added to `packing_list_handler.go`, next to `Delete`
  — there's no dedicated "archive handler" file to mirror the way
  `PackAll`/`UnpackAll` mirror each other in their own file, so a
  brand-new file for just this one action isn't warranted.
- **Edge cases**: same 4 as `Delete` (400 invalid id, 404 not found, 404
  not owned, 401 unauthenticated) — `requireOwnedPackingList` gives these
  for free, and `Delete`'s own tests are the direct precedent to mirror.

## Acceptance criteria

- [ ] `POST /lists/:id/unarchive` sets `archived_at` back to `NULL` for an
      archived list, returns `204 No Content`.
- [ ] Idempotent: calling it on an already-active list still returns `204`
      (no "already active" error), matching `ArchivePackingList`'s
      unconditional-UPDATE behavior.
- [ ] `400` for a non-UUID `:id`.
- [ ] `404` for a well-formed `:id` that doesn't exist.
- [ ] `404` for a list that exists but isn't owned by the caller.
- [ ] `401` for an unauthenticated request.
- [ ] Round-trip visible via existing list-visibility query: after
      unarchive, `GET /lists` (active) includes the list again and
      `GET /lists?archived=true` no longer does.

## Non-goals

- No change to `PackingList`/`PackingListDetail` JSON shape — `archivedAt`
  stays unexposed to clients, same as today; visibility is still only via
  which `GET /lists` view returns the list.
- No change to `DELETE /lists/:id` (archive direction) — it's untouched,
  only its inverse is being added.
- No hard-delete. Still soft-archive/unarchive only.
- Not touching PACK-030 (session-restore endpoint) — separate ticket,
  filed same day, no code overlap.

## Expected test files

- `internal/repository/packing_list_test.go` — add
  `TestUnarchivePackingList_ClearsArchivedAt` and
  `TestUnarchivePackingList_IdempotentOnAlreadyActive`, mirroring
  `TestArchivePackingList_SetsArchivedAt` / `TestArchivePackingList_Idempotent`
  (lines 501, 517) but asserting `archived_at` is `NULL` after. Use the
  existing `archivePackingListDirect` test helper (line 113) to set up an
  already-archived fixture.
- `internal/handler/packing_list_handler_test.go` — add
  `TestPackingListUnarchive_Success`,
  `TestPackingListUnarchive_IdempotentOnAlreadyActive`,
  `TestPackingListUnarchive_NotOwned`, `TestPackingListUnarchive_NotFound`,
  `TestPackingListUnarchive_InvalidID`,
  `TestPackingListUnarchive_Unauthorized` — mirror
  `TestPackingListDelete_Success` / `_NotOwned` / `_NotFound` / `_InvalidID`
  / `_Unauthorized` / `_IdempotentOnAlreadyArchived` (lines 641–718),
  swapping the mocked `ArchivePackingList` call for `UnarchivePackingList`
  and the method/path to `POST /lists/:id/unarchive`.
- `requests/packing_lists.http` — extend the existing `DELETE /lists/:id`
  section (ends at line 373) with a new `POST /lists/:id/unarchive`
  section: archive `createListNameOnly` (already done at line 353),
  unarchive it, then re-run the two `GET /lists`(`?archived=true`) checks
  from lines 365–373 to show the round-trip (back in the active view, out
  of the archived view).

## Stubs needed before implementation

- `PackingListRepository` interface
  (`internal/handler/packing_list_handler.go:13`): add
  `UnarchivePackingList(ctx context.Context, id string) error`.
- `PostgresPackingListRepository.UnarchivePackingList`: stub returning
  `errors.New("not implemented")`.
- `PackingListHandler.Unarchive(c *gin.Context)`: stub writing
  `http.StatusNotImplemented`.
- `MockPackingListRepository` (test file): add
  `UnarchivePackingList` mock method.
- `main.go`: register `authed.POST("/lists/:id/unarchive", packingListHandler.Unarchive)`
  next to the existing `DELETE /lists/:id` route.

## Close-out

Completed 2026-07-14. Retro entry in LESSONS.md.
