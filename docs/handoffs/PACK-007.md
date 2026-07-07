# PACK-007 — Item management

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from
> `internal/handler/item_handler.go`, `internal/handler/item_handler_test.go`,
> `internal/repository/item.go`, `internal/repository/item_test.go`, and the
> git log (`5f41aaf` "item test files" precedes `ada6bc9` "item handlers" —
> consistent with tests-first, though no handoff doc existed at the time).
> This is documentation of what was built, not a spec that preceded it.

## Context

Users create and manage items within categories — the building blocks that
templates (Epic 4) and packing lists (Epic 5) will reference.

## Acceptance criteria

- [x] `GET /items?category_id=&search=` — system + user-owned items.
  - [x] `category_id`, if given, must be a valid UUID for a category
        accessible to the caller (system or their own) — 400 otherwise.
  - [x] `search`, if given, does a case-insensitive partial match on name.
  - [x] Filters can be combined.
- [x] `POST /items` — creates a user-owned item.
  - [x] 400 on missing/empty/too-long name, missing `categoryId`, invalid
        `categoryId`, or a `categoryId` not accessible to the caller.
  - [x] 409 on duplicate name within the same category (unique per
        category, not globally — same name is fine in a different
        category).
  - [x] Items may be created under system categories, not just the user's
        own.
- [x] `PATCH /items/:id` — owner-only; can update name, category, or both.
  - [x] 400 if neither `name` nor `categoryId` is provided.
  - [x] 404 if the item isn't owned by the caller.
  - [x] 409 on rename/move collision (duplicate name in the target
        category).
- [x] `DELETE /items/:id` — owner-only.
  - [x] 409 if the item is referenced by a template or an active packing
        list (`ItemIsInUse`). Not yet exercisable end-to-end since
        templates/lists don't have handlers yet — the repository check
        exists and is covered by `repository/item_test.go`, but the .http
        file for this can't be completed until PACK-008/009 and Epic 5
        exist. Flagged in `requests/items.http`.

## Non-goals / files not touched by this ticket

- Categories are PACK-006.

## Tests

`internal/handler/item_handler_test.go`, `internal/repository/item_test.go`.
Manual request file: `requests/items.http`.
