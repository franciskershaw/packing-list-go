# PACK-033 — System categories seed idempotency

## Context

`db/seeds/categories.sql` has always used `ON CONFLICT DO NOTHING`, but
`categories.name` has no unique constraint — so there is no arbiter for
Postgres to detect a conflict against, and the seed silently no-ops into
always inserting. Flagged as a deferred non-goal in
`packing-list-react`'s `PACKFE-003` (`docs/specs/master-spec.md`) — "this
is Go-side work... becomes its own small `packing-list-go` ticket —
addressed when it actually blocks building/testing against real data."

That moment arrived: the dev database currently has zero categories, so
`packing-list-react`'s new `ItemFormModal` (PACKFE-003 Piece 4) has
nothing to populate its category-chip picker with, and no item can be
created without a category to assign it to. This blocks committing that
work safely.

**Scope, confirmed 2026-07-26**: categories only. A system-items seed was
also mentioned in the original deferred note, but isn't part of the
frontend's actual blocker (a user creates their own items through the
form regardless of whether any system items pre-exist) — filed
separately rather than bundled in here.

**Constraint shape**: a partial unique index on `categories(name) WHERE
user_id IS NULL` — scoped to system rows only. User-owned categories'
per-user uniqueness is already enforced at the application layer
(`CategoryNameExistsForUser`, case-insensitive) and is a separate
concern; this migration doesn't touch it.

**Testing**: manual verification only, no Go integration test. No
existing migration in this codebase has a Go test verifying it —
migrations are applied and checked manually, not unit-tested. This is
schema + seed-script plumbing, not application behavior, so it doesn't
fit the repository-test pattern (no new Go function is being added).

## Acceptance criteria

- [x] `db/migrations/000002_categories_system_name_unique.up.sql` adds
      `CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_system_name_unique
      ON categories (name) WHERE user_id IS NULL;`
- [x] Matching `.down.sql` drops the index
- [x] `db/seeds/categories.sql`'s `ON CONFLICT DO NOTHING` becomes
      `ON CONFLICT (name) WHERE user_id IS NULL DO NOTHING`, targeting the
      new index, plus a comment noting it's now safe to re-run
- [x] Manual verification (developer-run, against the real Neon dev DB):
      migration applied cleanly (`go run main.go`, `golang-migrate` ran
      automatically); seed re-run twice via Neon's web SQL Editor (not
      `psql` — not installed locally, see Non-goals below) left exactly
      12 category rows both times, not 24
- [ ] `packing-list-react`'s `ItemFormModal` (PACKFE-003 Piece 4) can
      actually select a category and create an item end-to-end — not
      re-verified here; categories now exist and are selectable, but the
      full click-through wasn't exercised in this session. Tracked as
      part of Piece 4's own manual-verification step in that project's
      `master-spec.md`, not double-tracked here.

## Non-goals

- No system-items seed (`db/seeds/items.sql`) — separate follow-up, not
  this ticket's blocker
- No change to user-owned category uniqueness (`CategoryNameExistsForUser`,
  `POST /categories`) — already enforced at the application layer,
  untouched by this migration
- No CI/automated seed-running step — the seed script stays a manually
  invoked SQL command, not wired into any automated pipeline. **Revised
  during manual verification**: assumed `psql` specifically; the
  developer doesn't have it installed, so it was run via Neon's web SQL
  Editor instead. That's now the default recommendation for ad hoc SQL
  against this project's (Neon-hosted) dev DB — see `LESSONS.md`,
  2026-07-26 — not a reason to install a local client.

## Expected test files

None (Go). Manual verification only, per the testing decision above:
re-ran `db/seeds/categories.sql` twice against the dev DB via Neon's web
SQL Editor and confirmed the row count held at 12 both times.

## Close-out

Completed 2026-07-26. Retro entry in LESSONS.md.
