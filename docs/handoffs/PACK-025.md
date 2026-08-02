# PACK-025 — DB indexes migration

One-line summary: add indexes on every FK/`user_id` column currently missing
one, so the audit finding's sequential scans become index scans as the
tables grow past trivial row counts.

## Context

Source finding: `docs/handoffs/audit-2026-07-11-findings.md` item 7 — "No
indexes beyond primary keys. Every FK column and every `WHERE user_id = $1`
query is a sequential scan. Irrelevant at current scale; a one-migration
fix."

**Design gate**: follows precedent, no ADR. `refresh_tokens` already has
`idx_refresh_tokens_user_id` (added in `000003_refresh_tokens.up.sql`) —
this ticket just extends that same `idx_<table>_<column>` naming pattern to
every other table. No existing ADRs in this project, so no revisit-when
trigger to check.

Grilled 2026-08-02. Most recent `LESSONS.md` entry at the time (2026-08-02,
PACK-024) didn't surface anything that changes this ticket's shape — its
lesson was about `golangci-lint`'s issue-capping defaults, orthogonal to a
schema migration.

Key decisions from the interview:

- **Scope**: index every FK column and every `user_id` column, not just the
  ones hit by a direct app-level `WHERE` clause. This also covers
  cascade-delete performance (e.g. deleting a template needs to scan
  `packing_lists.template_id`; deleting a category needs
  `packing_list_items.category_id`), not just read-path filtering.
  Confirmed against every `WHERE`/`JOIN` in `internal/repository/*.go`
  against the schema in `000001_init_schema.up.sql`.
- **Explicitly declined**: composite/expression indexes tuned to specific
  query shapes (e.g. `(user_id, lower(name))` on `categories`/`templates`
  to match the `WHERE LOWER(name) = LOWER($1) AND user_id = $2` uniqueness
  checks in `category.go`/`template.go`). Out of scope — goes beyond the
  finding's literal wording, and isn't a one-migration fix anymore once
  expression indexes are on the table.
- **`CREATE INDEX CONCURRENTLY`**: not used. The finding itself calls this
  "irrelevant at current scale," and `golang-migrate`'s postgres driver
  runs each migration inside a transaction, which `CREATE INDEX
  CONCURRENTLY` cannot run inside anyway.
- **Verification**: an automated test querying `pg_indexes` to assert each
  expected index exists, not a test asserting query-plan shape (`EXPLAIN`
  showing `Index Scan`). Confirmed via `internal/repository/main_test.go`
  that `db.InitDB` runs migrations against the real Neon dev DB before
  repository tests run, so this is a genuine integration check, not
  aspirational. An `EXPLAIN`-based test was explicitly rejected: Postgres'
  cost-based planner prefers `Seq Scan` on small tables regardless of
  index presence, so on this project's current tiny dev-DB row counts that
  assertion would likely false-negative even with a correctly created
  index — it would need seeded bulk fixture data to force the planner's
  hand, real added complexity for a scale explicitly called out as
  "irrelevant currently."
- **No `.http` file**: this ticket has no HTTP-facing behavior change (pure
  schema DDL), so the project's usual manual-verification artifact doesn't
  apply. Manual verification instead means a one-time `psql` spot-check
  (see Expected test files below), not asserted in CI.

## Acceptance criteria

- [ ] Migration `000005_add_missing_indexes` adds these 11 indexes:
  - [ ] `idx_categories_user_id` on `categories(user_id)`
  - [ ] `idx_items_category_id` on `items(category_id)`
  - [ ] `idx_items_user_id` on `items(user_id)`
  - [ ] `idx_templates_user_id` on `templates(user_id)`
  - [ ] `idx_template_items_template_id` on `template_items(template_id)`
  - [ ] `idx_template_items_item_id` on `template_items(item_id)`
  - [ ] `idx_packing_lists_user_id` on `packing_lists(user_id)`
  - [ ] `idx_packing_lists_template_id` on `packing_lists(template_id)`
  - [ ] `idx_packing_list_items_list_id` on `packing_list_items(list_id)`
  - [ ] `idx_packing_list_items_item_id` on `packing_list_items(item_id)`
  - [ ] `idx_packing_list_items_category_id` on `packing_list_items(category_id)`
- [ ] Each `CREATE INDEX` uses `IF NOT EXISTS`, matching the existing
      `idx_refresh_tokens_user_id` precedent.
- [ ] Down migration drops all 11 indexes with `DROP INDEX IF EXISTS`.
- [ ] `internal/repository/indexes_test.go` (new, hits the real Neon dev
      DB) asserts all 11 index names exist via `pg_indexes` after
      migrations run.
- [ ] `golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...`
      clean.

## Non-goals

- No composite or expression indexes (declined in interview — see Context).
- No changes to any query in `internal/repository/*.go`. This ticket only
  adds indexes; it does not rewrite queries to take advantage of them
  (there's nothing to rewrite — the existing queries already filter on the
  columns being indexed).
- No `CREATE INDEX CONCURRENTLY` (see Context).
- No changes to `refresh_tokens` — already indexed.
- Does not touch PACK-026 (OpenAPI spec) or PACK-028 (minor
  security/idiom cleanup) — both explicitly no functional coupling to this
  ticket per `master-spec.md`.

## Expected test files

- `internal/repository/indexes_test.go` — new. `package repository_test`,
  using the shared `db.DB` connection already established in
  `main_test.go`'s `TestMain`. Table-driven test over the 11
  `(table, index_name)` pairs above, querying
  `SELECT indexname FROM pg_indexes WHERE tablename = $1 AND indexname = $2`
  and asserting a row is found for each.
- Manual verification (one-time, not automated): after running the
  migration against the Neon dev DB, spot-check with `psql`:
  - `\d+ packing_lists` / `\d+ packing_list_items` to confirm the new
    indexes show up in each table's index list.
  - `EXPLAIN ANALYZE SELECT * FROM packing_lists WHERE user_id = '<a real
    user id>' AND archived_at IS NULL;` as a sanity check that the index is
    at least available to the planner, understanding the planner may still
    choose `Seq Scan` at current row counts — this step confirms the index
    was built correctly, not that it's currently being chosen.
