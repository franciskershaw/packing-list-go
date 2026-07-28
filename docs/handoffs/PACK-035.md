# PACK-035 — Delta bulk item mutations for lists and templates

## Context

Found 2026-07-28 while working through `packing-list-react`'s item-adding
modal, during a frontend session that redesigned it to hold local draft
quantities and flush them in one shot on "Done" instead of firing a request
per stepper click. That specific frontend redesign was itself later
discarded/reverted without committing (a separate, unrelated frontend
false-start) — so **this ticket is deliberately not pinned to any specific
frontend file's current shape or line numbers.** The current frontend is
back to a simpler per-click implementation (each `+`/increment click fires
its own request immediately) — same underlying problem, different specific
code path than what was originally observed. The architectural gap this
ticket fixes is independent of which frontend implementation is live: there
is no backend endpoint that lets a client add/update/remove several items
in one request with per-item quantities, so any frontend approach to
"edit several items, then save" — draft-and-flush or otherwise — is stuck
sending one request per item.

The existing category-based `POST /lists/:id/items/bulk` /
`POST /templates/:id/items/bulk` (`packing_list_item_handler.go:218-269`,
`template_item_handler.go:207-255`) don't help here — they only accept a
bare `categoryId` and add every item in it at a hardcoded quantity of 1,
with no per-item overrides. Their frontend usage is asymmetric and worth
being precise about, since this ticket deletes both: on the **list** side,
`useBulkAddTripItems` (`trips.ts:254-266`) has zero call sites — genuinely
dead code today. On the **template** side, `useBulkAddItems`
(`templates.ts:220-230`) **is** actively wired up — `TemplateAddItemsModal`'s
"+ All Camping"-style bulk-add button calls it for real, hitting
`POST /templates/:id/items/bulk`. Deleting that endpoint as this ticket
plans is therefore a **known, accepted breaking change** for templates:
that button will fail (404/405) the moment this ships, until a small
frontend follow-up switches it to build a delta payload client-side
instead (the frontend already holds the full item catalog via `useItems`,
so it can resolve "every item in this category" itself without server-side
help). Confirmed and explicitly accepted in chat rather than silently
shipping a regression — see the "Endpoint identity" decision below.

**Design-gate finding**: new pattern (first transactional multi-row mutation
endpoint driven by client input, and the first delta/upsert-or-delete
contract in this codebase) — normally flagged for a short ADR per the
global process, but explicitly declined in chat for this ticket: ADRs are a
convention adopted after this repo was mostly built, and retrofitting one
now for a single ticket would be inventing process for its own sake. Decision
recorded here instead, same as PACK-032/PACK-034 did for their own
new-pattern findings.

**Key decisions from the interview:**

- **Scope**: covers both `PackingListHandler` and `TemplateHandler` in one
  ticket. Confirmed identical bug in both during this session — same
  request/response shape, same repo-layer pattern, no shared code between
  them today (each has its own repository interface), so there's no
  isolation benefit to splitting into two tickets.
- **Contract shape — delta with implicit remove**, not a mixed
  action-tagged batch and not full-list replacement. Request body:
  `{ "items": [ { "itemId": "<uuid>", "quantity": <int> } ] }`. Only
  *changed* items need to be included — anything on the list/template not
  named in the payload is left untouched. Per entry: `quantity` in
  `[1, 999]` means add-if-absent-else-update; `quantity: 0` means
  remove-if-present, no-op if already absent (idempotent — the end state
  already matches what was asked for). Rejected two alternatives during
  the interview: an explicit `action: "add"|"update"|"remove"` discriminator
  (redundant — existence on the list already disambiguates add vs. update,
  and the client doesn't need to compute or send it), and full-list
  replacement where omission means delete (rejected as a real data-loss
  risk — any client bug that fails to include an item in the payload would
  silently delete it, versus delta where a bug can only affect items it
  explicitly names).
- **Quantity 0 is a new validation case.** The existing `validateQuantity`
  helper (`template_item_handler.go:257-265`) enforces `[1, 999]` for the
  single-item endpoints and is unchanged for them. The bulk endpoints need
  their own range check accepting `[0, 999]`, since `0` is a legal sentinel
  here specifically.
- **Atomicity: all-or-nothing, single DB transaction, no partial success.**
  All input validation (UUID format, quantity range, duplicate `itemId`
  within the request, item accessibility) happens *before* `BeginTx` — a
  malformed request never opens a transaction. Mirrors the existing
  transaction precedent in `CreatePackingList`
  (`internal/repository/packing_list.go:26-57`: `BeginTx` /
  `defer tx.Rollback()` / `tx.Commit()`), which this ticket is the second
  user of. A mid-transaction DB error rolls back the whole batch and
  returns 500, same as every other handler's DB-failure convention here.
  Explicitly not doing per-item partial-success reporting — for a
  personal-scale app, a failed batch can just be retried wholesale, and a
  per-item result shape would force new reconciliation logic on the
  frontend for no real payoff.
- **Malformed-payload handling**, all validated before the transaction
  opens: empty `items` array → 400 (mirrors `UpdateItem`'s existing "at
  least one of ... is required" check,
  `packing_list_item_handler.go:144-147`); duplicate `itemId` within the
  same request → 400, not last-write-wins.
- **Response: 204 No Content.** Whatever shape the frontend ends up in, a
  mutation like this only needs to report success/failure — returning the
  updated item set would be dead weight unless a specific frontend design
  is already committed to consuming it, which isn't the case here (see the
  note above about the frontend being in flux). Matches `RemoveItem`'s
  existing "mutation with nothing useful to return" precedent
  (`packing_list_item_handler.go:215`). Separately discussed whether
  returning data vs. 204 matters for UI responsiveness — it doesn't:
  optimistic updates (already used for exactly two mutations in this
  codebase's still-current `trips.ts`, `useUpdateTripItem`/`useBulkSetPacked`
  via `useOptimisticTripPatch`, `trips.ts:286-324` — confirmed still
  accurate, unlike the modal files above) patch the cache from the
  request's own variables in `onMutate`, never from the response body. If
  the frontend wants the bulk-add flow to feel instant, that's a follow-up
  frontend change mirroring that existing pattern — out of scope here.
- **No batch-size cap.** Considered as basic input sanity separate from
  rate-limiting, but left uncapped for now — explicitly deferred, not
  overlooked; revisit alongside PACK-022 (request-level abuse protection)
  if it ever becomes a real problem.
- **Endpoint identity: replace in place, method changes POST → PATCH.**
  `POST /lists/:id/items/bulk` and `POST /templates/:id/items/bulk` are
  deleted, not versioned or kept alongside. For the list side this is
  free — confirmed dead code, nothing depends on it. For the template side
  this is a **deliberate, accepted breaking change**: `useBulkAddItems` is
  a live caller today (see Context above), so templates' bulk-add-by-
  category UI will break until a frontend follow-up rebuilds it to send a
  client-resolved delta payload instead. Explicitly chosen over the two
  alternatives discussed — keeping the old categoryId endpoint alongside
  the new one (rejected: carries two bulk-add contracts for templates
  indefinitely), or pulling the frontend fix into this ticket (rejected:
  scope creep into a project that governs its own process — this ticket
  stays backend-only, matching the "no frontend changes" non-goal below).
  New method is PATCH, matching the existing single-item
  `PATCH /lists/:id/items/:itemId` (`UpdateItem`) semantically — this is
  "partially update the item collection," not "create a resource at a
  fixed path."
- **Supersedes part of PACK-028.** PACK-028's bundle includes "non-
  transactional `BulkAddItems`" as one of its items (finding 14,
  `audit-2026-07-11-findings.md`). Since `BulkAddItems` is deleted
  entirely by this ticket (not patched in place), that finding is fully
  resolved here — PACK-028's own scope should drop it when PACK-028 is
  picked up.
- **New repo method needed for accessibility checks**:
  `ItemRepository.GetItemsByIDs(ctx, ids []string) ([]models.Item, error)`
  (`internal/repository/item.go`), exposed through the existing
  `ItemLookupRepository` interface (`template_handler.go:31-35`) so both
  handlers can validate every `itemId` in a batch in one query instead of
  N single lookups — mirrors `GetItems`' existing style
  (`internal/repository/item.go:21`), just filtered by
  `WHERE id = ANY($1)` instead of category/search.
- No migration — no schema change, this is handler/repository logic only.

## Acceptance criteria

**Repository layer (own commit, pause for review before the handler layer
— per this project's layer-by-layer implementation rule):**

- [ ] `ItemRepository.GetItemsByIDs(ctx, ids []string) ([]models.Item, error)`
      returns every item matching the given IDs (accessible or not — handler
      does the accessibility filtering, same division of responsibility as
      `GetItemByID` today)
- [ ] `PackingListRepository.BulkUpdatePackingListItems(ctx, listID string,
      changes map[string]int) error` — within a single transaction
      (mirroring `CreatePackingList`'s `BeginTx`/`defer Rollback`/`Commit`
      pattern): inserts items absent from the list with `quantity > 0`,
      updates items present on the list whose quantity differs, deletes
      items present on the list with `quantity == 0`, no-ops for entries
      that are already in their target state
- [ ] `TemplateRepository.BulkUpdateTemplateItems(ctx, templateID string,
      changes map[string]int) error` — identical shape, targeting
      `template_items`
- [ ] A mid-batch DB failure (e.g. a query against a dropped connection)
      leaves the list/template's items completely unchanged (transaction
      rollback verified via a real integration test, not just code
      inspection)
- [ ] The old `BulkAddItems`-supporting repo code path (the categoryId-driven
      loop in the handler — see below) is removed; no repo method existed
      solely for it beyond the already-shared `AddPackingListItem`/
      `AddTemplateItem`, so no repo-layer deletion is expected here beyond
      what's superseded by the two new methods above

**Handler layer:**

- [ ] `PATCH /lists/:id/items/bulk` replaces the old
      `POST /lists/:id/items/bulk` (route method updated in `main.go:112`);
      `PackingListHandler.BulkAddItems` is renamed/replaced by
      `BulkUpdateItems` implementing the delta contract
- [ ] `PATCH /templates/:id/items/bulk` replaces the old
      `POST /templates/:id/items/bulk` (`main.go:101`);
      `TemplateHandler.BulkAddItems` replaced the same way
- [ ] Empty `items` array → 400
- [ ] Duplicate `itemId` within one request → 400, no partial write
- [ ] Any `itemId` malformed (not a valid UUID) → 400
- [ ] Any `itemId` not accessible to the caller (system/owned check, mirrors
      `isItemAccessible`) → 400, no partial write
- [ ] Any `quantity` outside `[0, 999]` → 400, no partial write
- [ ] `quantity: 0` for an item not currently on the list/template → no-op,
      not an error
- [ ] A fully valid batch mixing add/update/remove in one request commits
      atomically and returns `204 No Content`
- [ ] A DB-layer failure mid-batch returns 500 and leaves prior state intact
      (asserted via a mocked repository returning an error, at the handler
      level — the real rollback behavior is the repo-layer test above)
- [ ] Both list and template handler tests cover the same set of cases
      (they're independent handlers, no shared test helper currently unifies
      them, and this ticket isn't introducing one)

## Non-goals

- No ADR (see design-gate finding above — explicitly declined in chat)
- No batch-size cap (explicitly deferred to alongside PACK-022, not
  overlooked)
- No partial-success / per-item result reporting
- No change to the single-item `AddItem`/`UpdateItem`/`RemoveItem` endpoints
  on either resource — inline edit-mode single-item interactions are
  unaffected and out of scope
- No frontend changes. Note this is not fully consequence-free: templates'
  "+ All Camping"-style bulk-add button currently calls the endpoint this
  ticket deletes (`useBulkAddItems`) and will break until a frontend
  follow-up switches it to a client-resolved delta payload — an accepted,
  known break, not an oversight (see Context and the "Endpoint identity"
  decision above). The list side has no equivalent live caller, so no
  break there. Whether/when that frontend follow-up happens, and whether it
  needs a formal ticket, is governed by `packing-list-react`'s own CLAUDE.md
  (that branch has no mandatory handoff-doc process).
- No optimistic-UI design work — noted as a possible frontend follow-up
  only, mirroring the existing `useOptimisticTripPatch` pattern
- No OpenAPI spec update (PACK-026, separate ticket, not started)
- No change to `notes`/`sortOrder`/`isPacked` handling — the bulk endpoints
  only ever touch `quantity`; those fields stay single-item-only

## Expected test files

- `internal/repository/packing_list_item_test.go` — new
  `TestBulkUpdatePackingListItems_AddsUpdatesAndRemoves` (integration test
  against the real Neon dev DB: seed a list with a couple of items, send a
  batch mixing a new add, an update to an existing item, and a removal of
  another existing item, assert final state via `GetPackingListItems`); new
  `TestBulkUpdatePackingListItems_RollsBackOnFailure` (force a failure
  partway — e.g. a second call against a canceled context — and assert no
  partial writes landed).
- `internal/repository/template_item_test.go` — same two tests, mirrored for
  `BulkUpdateTemplateItems`/`GetTemplateItems`.
- `internal/repository/item_test.go` — new `TestGetItemsByIDs_ReturnsMatches`
  (mirrors the existing `TestGetItems_FilterByCategory` style).
- `internal/handler/packing_list_item_handler_test.go` — new
  `TestBulkUpdateItems_EmptyArray`, `TestBulkUpdateItems_DuplicateItemId`,
  `TestBulkUpdateItems_InvalidQuantity`, `TestBulkUpdateItems_InaccessibleItem`,
  `TestBulkUpdateItems_NoopRemoveOfAbsentItem`,
  `TestBulkUpdateItems_MixedBatchSucceeds`,
  `TestBulkUpdateItems_RepoErrorReturns500` — all against a mocked repo per
  this project's `testify/mock` convention, using the shared `doRequest`
  helper (`handler_test_helpers_test.go`).
- `internal/handler/template_item_handler_test.go` — same set, mirrored for
  `TemplateHandler`.
- `requests/packing_lists.http` — replace the existing
  `POST /lists/:id/items/bulk` section (lines ~679-752) with a `PATCH`
  version demonstrating a mixed add/update/remove batch, plus the malformed
  cases (empty array, duplicate itemId, out-of-range quantity).
- `requests/template_items.http` — same replacement for the
  `POST /templates/:id/items/bulk` section (lines ~278-337).
