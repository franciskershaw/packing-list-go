# PACK-006 — Category management

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from
> `internal/handler/category_handler.go`,
> `internal/handler/category_handler_test.go`,
> `internal/repository/category.go`, `internal/repository/category_test.go`,
> and the git log (`d080e40` "Tests and seed" precedes `dd4b921` "category
> handlers" — consistent with tests-first, though no handoff doc existed at
> the time). This is documentation of what was built, not a spec that
> preceded it.

## Context

Authenticated users can see the shared system category list and manage
their own categories alongside it.

## Acceptance criteria

- [x] `GET /categories` — returns system categories (`user_id IS NULL`)
      plus the caller's own, as a flat list. 401 with no token.
- [x] `POST /categories` — creates a user-owned category.
  - [x] 400 on missing/empty name, or name over 100 characters
        (`validateName` in `internal/handler/validation.go`).
  - [x] 409 if the user already has a category with that name
        (`CategoryNameExistsForUser`). Same name as a *system* category is
        allowed.
- [x] `PATCH /categories/:id` — renames a category the caller owns.
  - [x] 400 on invalid UUID or invalid name.
  - [x] 404 if the category doesn't exist, is system-owned, or belongs to
        another user (ownership is checked via `isOwned`/`isOwnedBy` —
        deliberately not distinguished from "doesn't exist" to avoid
        leaking existence of other users' data).
  - [x] 409 on rename collision with another of the caller's categories.
- [x] `DELETE /categories/:id` — deletes a category the caller owns.
  - [x] Same 400/404 rules as PATCH.
  - [x] 409 if the category still has items under it
        (`CategoryHasItems`).

## Non-goals / files not touched by this ticket

- Items themselves are PACK-007.

## Tests

`internal/handler/category_handler_test.go`,
`internal/repository/category_test.go`. Manual request file:
`requests/categories.http`.
