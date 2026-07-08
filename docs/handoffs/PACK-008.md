# PACK-008 — Template CRUD

## Context

Templates are named, reusable lists of items (e.g. "Weekend hiking") that a
user assembles from their item library and later seeds packing lists from
(PACK-010). This is the first ticket built under the real pipeline — a prior
attempt left `internal/models/template.go` and
`internal/repository/template.go` half-built (model + stubbed repo methods
returning `errors.New("not implemented")`, no handler, no tests, nothing
wired into `main.go`). That gap is what prompted the `grill-me` /
handoff-doc process (see `LESSONS.md`, 2026-07-07 entry). This ticket
replaces the stubs with real implementations, test-first.

Unlike categories/items, templates have no system-level concept — the
`templates` table has `user_id UUID NOT NULL`, so every template is owned by
exactly one user. Ownership checks are still 404-on-mismatch (not 403), same
as categories/items.

Key decisions from the interview:

- **Name uniqueness**: enforced, same as categories — case-insensitive,
  scoped per user, 409 on conflict. Checked on both create and update
  (excluding self on update).
- **PATCH semantics**: no explicit null-vs-omit distinction. `UpdateTemplate`
  drops the `descriptionSet` param that was in the stub — `description
  *string` alone; omitted key or `null` both mean "leave unchanged" (both
  decode to `nil`), and sending `description: ""` stores an empty string,
  not `NULL`. At least one of `name`/`description` must be present in the
  body (400 if neither), matching the item PATCH pattern.
- **Description validation**: optional, trimmed, capped at 500 chars (400 if
  over). New `validateDescription` helper alongside the existing
  `validateName` (100 chars, required).
- **Delete**: unconditional, no usage check. `packing_lists.template_id` is
  `ON DELETE SET NULL`, and the spec lists no blocking condition for
  templates (unlike categories/items, which 409 when in use).
- **List ordering**: `GET /templates` orders by `updated_at DESC` (most
  recently touched first), not alphabetically — templates are more
  work-in-progress than the item library.
- **Items field scope**: `Template.Items` is hard-coded to `[]TemplateItem{}`
  in both `GetTemplates` and `GetTemplateByID` for this ticket. The real
  join against `template_items`/`items` is PACK-009's job, once there's a
  way to populate rows. No `template_items` rows can exist yet regardless.

## Acceptance criteria

- [ ] `GET /templates` — returns only the caller's templates, ordered by
      `updated_at DESC`. `items` is always `[]` in the response.
  - [ ] 401 if unauthenticated.
- [ ] `POST /templates` — creates a template owned by the caller.
  - [ ] `name` required, trimmed, ≤100 chars — 400 otherwise.
  - [ ] `description` optional, trimmed, ≤500 chars — 400 if over.
  - [ ] 409 if the (trimmed, case-insensitive) name already exists for this
        user.
  - [ ] 201 with the created template (`items: []`) on success.
- [ ] `GET /templates/:id` — returns the template with its items.
  - [ ] 400 on invalid UUID.
  - [ ] 404 if the template doesn't exist or isn't owned by the caller.
  - [ ] `items` is always `[]` (join deferred to PACK-009).
- [ ] `PATCH /templates/:id` — updates name and/or description.
  - [ ] 400 if neither `name` nor `description` is present in the body.
  - [ ] Same name/description validation as create.
  - [ ] 404 if not found / not owned.
  - [ ] 409 on name collision with another of the caller's templates
        (excluding itself).
  - [ ] 200 with the updated template on success.
- [ ] `DELETE /templates/:id` — deletes unconditionally.
  - [ ] 400 on invalid UUID.
  - [ ] 404 if not found / not owned.
  - [ ] 204 on success. No usage/in-use check.
- [ ] Routes registered in `main.go` under the existing authenticated group,
      alongside categories/items.

## Non-goals / files not touched by this ticket

- Adding, updating, removing, or bulk-adding items on a template
  (`POST/PATCH/DELETE /templates/:id/items*`) — PACK-009.
- Implementing the real `template_items` join in `GetTemplateByID` /
  `GetTemplates` — deferred to PACK-009 (see decision above).
- Packing list creation or seeding from a template — PACK-010.
- Any change to `categories`/`items` handlers, repos, or their tests.
- `internal/models/template.go`'s `TemplateItem` struct shape — unchanged.

## Expected test files

- `internal/repository/template_test.go` — real-DB integration tests
  (package `repository_test`, following `category_test.go`'s pattern):
  `GetTemplates` (scoping to caller, `updated_at DESC` ordering, empty
  slice for a user with none), `CreateTemplate`, `GetTemplateByID` (found /
  `nil` for missing), `UpdateTemplate` (name only, description only, both),
  `DeleteTemplate`, `TemplateNameExistsForUser` (with and without
  `excludeID`, case-insensitivity).
- `internal/handler/template_handler_test.go` — mocked-repo handler tests
  (package `handler_test`, `testify/mock`, following
  `category_handler_test.go`'s `MockCategoryRepository` pattern): List
  (success, unauthorized), Create (success, missing/too-long name,
  too-long description, name conflict), GetByID (success, invalid UUID,
  not found, wrong owner), Update (success on name, success on
  description, 400 when both omitted, conflict, not found/wrong owner),
  Delete (success, invalid UUID, not found/wrong owner).
- `requests/templates.http` — manual verification requests, mirroring
  `requests/categories.http` / `requests/items.http`.
