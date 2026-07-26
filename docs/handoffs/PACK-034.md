# PACK-034 — Template list item counts

## Context

`packing-list-react`'s `PACKFE-004` (Templates screen) has Piece 6 (list/rail
assembly) next up, and every screenshot for that screen shows an item count
on each template row. `GetTemplates`' `scanTemplate` helper always leaves
`Items: []` — only `GetTemplateByID` (the detail fetch) populates items, via
a second query — confirmed by reading `internal/repository/template.go`'s
current source. There's currently nothing to derive a count from on the list
endpoint without a second per-template round trip.

Found 2026-07-26 during `packing-list-react` frontend work and flagged in
that project's `PACKFE-004` Architecture entry as its own small
`packing-list-go` ticket — same precedent PACK-033 set for the
category-seeding gap. Logged here as its own ticket in Epic 7
("Production Readiness & Frontend Bridge").

**Design-gate finding**: this is a new pattern, not a followed precedent —
no existing repository method anywhere in this codebase populates a
computed/aggregate field (`grep -rn "COUNT(" internal/repository/
internal/models/` returns nothing). `PackingList`'s own list endpoint
(`GetPackingLists`) has the identical gap and Trips will likely want the
same treatment later, but no ADR is being written for this — decided during
grill-me to record the decision here instead, judged small enough for a
handoff doc to carry (same call PACK-032 made for its own new-pattern
finding).

**Key decisions from the interview:**

- `ItemCount int` (`json:"itemCount"`) is added directly to the shared
  `models.Template` struct — used by both `GetTemplates` (list) and
  `GetTemplateByID` (detail) — rather than a separate list-only response
  type. Unlike `PackingList`/`PackingListDetail`'s existing split (which
  exists because those two shapes are genuinely different — flat vs.
  category-grouped), this is just one extra field; forking a near-duplicate
  type for it would be over-engineering.
- `GetTemplateByID` sets `ItemCount = len(items)` after its existing second
  query populates `Items` — free, no SQL change needed there.
- `GetTemplates` computes the count via a correlated subquery in `SELECT`:
  `(SELECT COUNT(*) FROM template_items WHERE template_id = templates.id)
  AS item_count` — avoids `GROUP BY` column-list/`ORDER BY` interaction
  gotchas a `LEFT JOIN ... GROUP BY` would introduce, and there's no
  existing precedent pulling toward either style.
- `scanTemplate()` stays untouched — still used by `GetTemplateByID`,
  `CreateTemplate`, `UpdateTemplate`, where `ItemCount` defaults to Go's
  zero value (`0`) except `GetTemplateByID`'s explicit override above.
  `GetTemplates` gets its own bespoke inline scan loop (extra column, a
  different signature) rather than generalizing the shared helper to
  handle a column not every caller's query selects.
- `CreateTemplate`/`UpdateTemplate` responses may carry a stale/zero
  `ItemCount` — confirmed against `packing-list-react`'s
  `useCreateTemplate`/`useUpdateTemplate`: both invalidate the broader
  templates list query rather than trusting the mutation's own response
  for count display, so this is harmless, not a gap to close here.
- No migration — this is query logic only, no schema change.
- Manual verification uses Neon's web SQL Editor if any ad hoc query is
  needed, not `psql` — per the `LESSONS.md` precedent from PACK-033 (no
  local `psql` client on this machine).

## Acceptance criteria

**Repository layer (own commit, pause for review before the handler layer
— per this project's layer-by-layer implementation rule):**

- [ ] `models.Template` gains `ItemCount int` (`json:"itemCount"`)
- [ ] `GetTemplateByID` sets `ItemCount` to `len(items)` after fetching them
- [ ] `GetTemplates` returns the correct `ItemCount` per template via a
      correlated `COUNT` subquery, with no second per-template round trip
- [ ] A template with zero items returns `ItemCount: 0` from `GetTemplates`
      (not omitted, not null)
- [ ] `CreateTemplate`/`UpdateTemplate` continue to work unchanged (their
      `ItemCount` may be `0`/stale — see decision above, not a defect)

**Handler layer:**

- [ ] `GET /templates` JSON response includes `itemCount` for each template
- [ ] `GET /templates/:id` also includes `itemCount` (harmless, no current
      consumer relies on it there)

## Non-goals

- No change to `GET /templates/:id`'s items population — `Items` stays as-is
- No change to `PackingList`'s identical list-endpoint gap
  (`GetPackingLists`) — a separate future item if/when it blocks something,
  not bundled into this ticket
- No OpenAPI spec update (PACK-026, separate ticket, not started)
- No `packing-list-react` changes — that's PACKFE-004 Piece 6, picked up
  once this ticket ships
- No ADR (see design-gate finding above)

## Expected test files

- `internal/repository/template_test.go` — new `TestGetTemplates_ItemCount`
  (integration test against the real Neon dev DB, using the existing
  `createTestTemplate`/`createTestCategory`/`createTestItem` helpers +
  `templateRepo.AddTemplateItem` to seed items, asserting the returned
  `ItemCount` matches); extend `TestGetTemplateByID_Found` to also assert
  `ItemCount == 0` for a fresh template with no items.
- `internal/handler/template_handler_test.go` — new
  `TestTemplateList_IncludesItemCount` (mocked repo returning a `Template`
  with a nonzero `ItemCount`, asserting the JSON body's `itemCount` field).
- `requests/templates.http` — extend the existing `GET /templates` section
  with a request demonstrating a nonzero `itemCount` after adding an item
  (via `template_items.http`'s `POST /templates/:id/items`, or inline in
  this file).
