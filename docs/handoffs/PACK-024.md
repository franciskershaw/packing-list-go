# PACK-024 — CI pipeline + fail-loud DB tests

## Context

Source: `docs/handoffs/audit-2026-07-11-findings.md` item 6. Two distinct
problems, one ticket:

1. **No CI pipeline exists.** No `.github/workflows/`, no Dockerfile, no
   lint config anywhere in this repo. `gofmt`/`go vet`/`golangci-lint`/
   `govulncheck`/`go test` all currently run only if a human remembers to
   run them locally.
2. **`internal/repository/main_test.go`'s `TestMain` silently skips
   instead of failing.** Lines 23-27 today:
   ```go
   if os.Getenv("DATABASE_URL") == "" {
       fmt.Println("skipping repository tests: DATABASE_URL not set")
       os.Exit(0)
   }
   ```
   Exit 0 with zero tests run reports as a clean pass. This is the
   structural version of the exact incident in `LESSONS.md`'s 2026-07-10
   (PACK-018) entry, where `DATABASE_URL=$DATABASE_URL go test
   ./internal/repository/...` silently expanded to empty in the user's own
   shell and the repository suite hadn't actually run against Neon for at
   least one prior ticket — caught only because 107 tests completing in
   0.25s was implausible. That was a shell-quoting incident; this ticket
   makes the underlying gate impossible to slip past silently, in any
   shell, with or without CI.

**Design gate**: new pattern, first CI pipeline this project has ever had.
No existing ADR touches CI. Considered writing one — declined: the
individual choices below (GitHub Actions, default `golangci-lint` config,
real Neon dev DB via secret, env-var opt-out) are conventional and
reversible, not the kind of hard-to-undo commitment (auth model, data
model) the ADR process exists for. This doc's Context section is the
record.

**Scope split, decided in this grill-me**: this ticket does **not**
include a Dockerfile or a deployment target — the audit finding it traces
to only names CI gates and the DB-test fix, and no containerization/
deployment artifact exists in this repo today. Filed separately as
**PACK-038** (`docs/specs/master-spec.md`, Epic 7).

Key decisions from interview:

- **CI runs the real repository suite against the real Neon dev DB.**
  `DATABASE_URL` is added as a GitHub Actions repo secret (same value as
  local dev), so `go test ./...` in CI exercises `internal/repository/...`
  for real, not just `internal/handler/...`. Adding the secret in GitHub's
  repo settings is a manual step for the user to do themselves (not
  something this ticket's implementation, or this session, touches — it's
  a live credential, not code).
- **Fail-loud mechanism**: `ALLOW_SKIP_DB_TESTS=1` env var, explicit
  opt-in to skip. Missing `DATABASE_URL` without that flag set is
  `os.Exit(1)` with a `FATAL` message; with the flag set, behavior is
  unchanged from today (prints a skip message, `os.Exit(0)`).
- **Workflow triggers**: `push` (any branch) and `pull_request`.
- **`golangci-lint`**: no `.golangci.yml`. Runs with its own built-in
  default linter set via `golangci-lint-action`. A curated lint policy is
  explicitly deferred, not decided now.
- **Go version**: pin `actions/setup-go` via `go-version-file: go.mod`
  (currently `go 1.26.2`) rather than hardcoding a version number, so CI
  and `go.mod` can't drift apart silently.
- **Testing the fail-loud behavior itself is automated, not just manual.**
  `TestMain`'s `os.Exit` can't be asserted from a test in the same binary
  (it would kill the test runner before any assertion ran), so this needs
  a subprocess test: `exec.Command("go", "test",
  "./internal/repository/...")` with `DATABASE_URL` stripped from the
  child's env, asserting exit code 1 and stderr containing `FATAL`; a
  second case sets `ALLOW_SKIP_DB_TESTS=1` and asserts exit code 0.
  **Placement matters**: this test must NOT live inside
  `internal/repository` itself — if it did, running `go test
  ./internal/repository/...` would spawn a child process that runs `go
  test ./internal/repository/...`, which spawns another, unboundedly. It
  lives at the repo root (`package main`, alongside the existing
  `main_test.go`, but as its own new file since it's a distinct concern
  from that file's server-lifecycle tests) — a different package than the
  one it execs, so no recursion.

## Acceptance criteria

- [ ] AC1 — `.github/workflows/ci.yml` exists, triggered on `push` and
      `pull_request`, running in this order: `gofmt -l .` (fails if
      non-empty output), `go vet ./...`, `golangci-lint-action` (default
      config), `govulncheck ./...`, `go test ./...` (with `DATABASE_URL`
      from a repo secret in the job's env, so the repository suite runs
      for real). Go version pinned via `go-version-file: go.mod`.
- [ ] AC2 — `internal/repository/main_test.go`'s `TestMain`: missing
      `DATABASE_URL` without `ALLOW_SKIP_DB_TESTS=1` set is `os.Exit(1)`
      with a `FATAL: DATABASE_URL not set...` message printed to stdout.
      Missing `DATABASE_URL` with `ALLOW_SKIP_DB_TESTS=1` set keeps
      today's behavior (skip message, `os.Exit(0)`). `DATABASE_URL` set
      is unaffected either way.
- [ ] AC3 — a new root-level test file (e.g. `db_failloud_test.go`,
      `package main`) has a subprocess test asserting both branches of
      AC2's behavior via `exec.Command`, per the Context note on
      placement (must not live inside `internal/repository`).

## Non-goals

- Any Dockerfile, container image, or deployment-target selection —
  **PACK-038**, filed separately.
- A curated `.golangci.yml` linter policy — explicitly deferred to a
  future pass once default output has been seen in real CI runs.
- Adding the `DATABASE_URL` GitHub Actions secret itself — a manual step
  for the user in GitHub's repo settings, not part of this ticket's
  implementation.
- Any change to `db/migrations/`, `internal/repository/*.go`'s actual
  query logic, or any handler — this ticket only touches CI config and
  `main_test.go`'s `TestMain`.
- PACK-025 (DB indexes) and PACK-026 (OpenAPI spec) — separate tickets,
  untouched here despite being adjacent in the backlog.

## Expected test files

- `db_failloud_test.go` (new, repo root, `package main`):
  - `TestRepositorySuite_FailsLoudWithoutDatabaseURL` — subprocess run
    with `DATABASE_URL` stripped, `ALLOW_SKIP_DB_TESTS` unset, asserts
    exit code 1 and `FATAL` in output.
  - `TestRepositorySuite_SkipsWithOptOutFlag` — subprocess run with
    `DATABASE_URL` stripped, `ALLOW_SKIP_DB_TESTS=1` set, asserts exit
    code 0.
- `internal/repository/main_test.go` (modified, not a new file) — the
  `TestMain` fail-loud change itself; no new test functions here, this is
  the behavior AC3's subprocess tests exercise from outside.
- Manual verification: push a branch (or open a PR) against the real
  `franciskershaw/packing-list-go` GitHub remote once `DATABASE_URL` is
  added as a repo secret; confirm the Actions run shows all five steps
  passing. Then deliberately break one gate (e.g. an unformatted file) on
  a throwaway commit and confirm the workflow fails at that specific step,
  before reverting it. Not a `.http` file addition — this ticket has no
  new HTTP-reachable behavior.

## Close-out

Not started.
