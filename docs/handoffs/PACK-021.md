# PACK-021 — Server lifecycle hardening

## Context

Source: `docs/handoffs/audit-2026-07-11-findings.md` items 1-2 (no
`http.Server` timeouts — slowloris exposure, no graceful shutdown; Gin
runs in debug mode unless `GIN_MODE=release` is set from
`cfg.Environment`).

Scope also includes the follow-up bundled in from PACK-027's grill-me
(`docs/specs/master-spec.md`'s PACK-021 entry, added 2026-08-01): once
graceful shutdown exists, replace the placeholder for a global periodic
sweep of expired/revoked `refresh_tokens` rows with a real `time.Ticker`
goroutine, coordinated against shutdown. PACK-027 deliberately built only
a lazy per-user sweep on login (`DeleteStaleFamiliesForUser`, fired from
`GoogleCallback`) because no shutdown coordination existed yet to stop a
ticker against — see `docs/handoffs/PACK-027.md` lines 97-108.

Design gate: timeouts/graceful-shutdown/release-mode are standard Go/Gin
idioms, no project precedent needed. The ticker is the first
background-goroutine pattern in this codebase — considered whether it
warranted a short ADR given future background jobs will likely copy its
shape; **decided against an ADR**, documenting the decision here instead
(small enough, and the interval/interface/wait-group approach chosen
below is simple enough to read as self-explanatory from source).

Key decisions from interview:

- **Timeouts**: `ReadHeaderTimeout` 5s, `ReadTimeout` 10s, `WriteTimeout`
  15s, `IdleTimeout` 60s. Generous enough not to clip a real client on a
  normal request, tight enough to free connections/goroutines held by
  abandoned ones. `ReadHeaderTimeout` is the specific slowloris mitigation
  from finding 1.
- **Graceful shutdown**: `signal.NotifyContext(context.Background(),
  os.Interrupt, syscall.SIGTERM)`; on trigger, `httpServer.Shutdown(ctx)`
  with a 10s grace-period context. SIGTERM covers Docker/systemd/deploy
  platform restarts; SIGINT covers local Ctrl+C.
- **Gin mode**: `gin.ReleaseMode` iff `cfg.Environment == "production"`,
  else `gin.DebugMode`. Matches the existing `APP_ENV == "production"`
  convention already used by `scripts/gen_token.go`'s production guard
  (audit finding 13) — an unrecognized/misconfigured `Environment` value
  falls back to the more-verbose `DebugMode`, not the less-safe direction.
- **Sweep interval**: every 1 hour. Refresh tokens live 7 days, so an hour
  of staleness before cleanup is harmless, and hourly keeps the sweep
  query infrequent.
- **New repo method**: `DeleteAllStaleFamilies(ctx) error` on
  `PostgresRefreshTokenRepository` — same `WHERE (revoked_at IS NOT NULL
  OR expires_at < CURRENT_TIMESTAMP)` clause as the existing
  `DeleteStaleFamiliesForUser` (`internal/repository/refresh_token.go:
  84-86`), minus the `user_id` filter. Purely additive; the lazy
  per-user sweep on login stays as-is.
- **Sweeper interface**: `type tokenSweepRepository interface {
  DeleteAllStaleFamilies(ctx context.Context) error }` defined in
  `main.go` (package `main` is the consumer) — extends this project's
  existing consumer-defined-interface convention (currently applied to
  the `handler` package) to `main`.
- **Shutdown coordination**: `main()` restructured to build `*http.Server`
  explicitly (replacing `server.Run()`, Gin's blocking shorthand with no
  `Shutdown()` access) and run `ListenAndServe` in a goroutine. The ticker
  goroutine (`runTokenSweeper`) is tracked with a `sync.WaitGroup`; it
  loops on `select { case <-ticker.C: sweep; case <-ctx.Done(): return }`
  and calls `wg.Done()` on exit. Sequence on signal: `<-ctx.Done()` →
  `httpServer.Shutdown(shutdownCtx)` (10s) → `wg.Wait()` → fall through to
  the existing deferred `db.CloseDB()`. Guarantees the sweep can't fire
  against a closing DB connection pool during shutdown.
- **Extraction scope**: only the 3 new pieces are pulled into testable
  functions — `configureGinMode(env string)`, `newHTTPServer(addr string,
  handler http.Handler) *http.Server`, `runTokenSweeper(ctx,
  tokenSweepRepository, interval, *sync.WaitGroup)`. Route registration
  and the rest of `main()` stay untouched — full extraction
  (`newRouter(deps...) *gin.Engine`) is PACK-028 item 16, not this
  ticket's scope.
- **Manual verification form**: no new `requests/*.http` file — this
  ticket adds no HTTP endpoint, so there's no request/response contract
  to regression-test. A documented curl/kill sequence in this doc instead
  (see below).

## Acceptance criteria

- [ ] AC1 — `newHTTPServer` constructs `*http.Server` with
      `ReadHeaderTimeout=5s`, `ReadTimeout=10s`, `WriteTimeout=15s`,
      `IdleTimeout=60s`; `main()` uses it (via `ListenAndServe` in a
      goroutine) instead of `server.Run()`.
- [ ] AC2 — `configureGinMode(env string)` sets `gin.ReleaseMode` iff
      `env == "production"`, else `gin.DebugMode`; called from `main()`
      with `cfg.Environment` before the Gin engine is constructed.
- [ ] AC3 — `PostgresRefreshTokenRepository.DeleteAllStaleFamilies(ctx)
      error` removes every revoked-or-expired `refresh_tokens` row across
      all users (repository layer — own commit, own review pause, before
      AC4).
- [ ] AC4 — Graceful shutdown wired: `signal.NotifyContext` (SIGINT,
      SIGTERM) → on trigger, `httpServer.Shutdown` with a 10s grace
      context, then `wg.Wait()` for `runTokenSweeper` to stop, then the
      existing deferred `db.CloseDB()`. `runTokenSweeper` calls
      `DeleteAllStaleFamilies` once per hour and stops cleanly (no leak,
      no post-cancellation tick) when its context is cancelled.

## Non-goals

- Extracting `main()`'s route registration into `newRouter(deps...)
  *gin.Engine` — PACK-028 item 16.
- Rate limiting, request body size cap, `SetTrustedProxies(nil)` —
  PACK-022.
- OAuth state map bound/sweep fix — PACK-023.
- Structured logging beyond the existing `slog` setup in `main.go` —
  PACK-028 item 15.
- Making the sweep interval or shutdown grace period configurable via
  env/config — hardcoded constants per the interview decisions above, not
  asked for.
- No ADR (explicit decision, see Context).
- No new `requests/*.http` file (explicit decision, see Context).
- Changing `DeleteStaleFamiliesForUser` or the lazy per-user sweep on
  login — stays exactly as PACK-027 shipped it.

## Expected test files

- `main_test.go` (package `main` — first test file at this level; `main`
  can't be black-box tested since it isn't importable):
  - `TestConfigureGinMode_ProductionSetsReleaseMode`
  - `TestConfigureGinMode_NonProductionSetsDebugMode` (covers both
    `"development"` and an unrecognized value, falling back to
    `DebugMode`)
  - `TestNewHTTPServer_SetsConfiguredTimeouts` — field-equality assertion
    against the constructed `*http.Server`.
  - `TestRunTokenSweeper_CallsDeleteAllStaleFamiliesOnEachTick` — mocked
    `tokenSweepRepository` (testify/mock, consistent with the existing
    handler-test override), short interval, asserts `.On("
    DeleteAllStaleFamilies", ...)` fires more than once before cancel.
  - `TestRunTokenSweeper_StopsCleanlyOnContextCancellation` — cancels
    context, asserts `wg.Done()` fires (via `wg.Wait()` returning) and no
    further mock calls happen after cancellation.
- `internal/repository/refresh_token_test.go` — add
  `TestDeleteAllStaleFamilies_RemovesRevokedOrExpiredAcrossUsers`,
  mirroring `TestDeleteStaleFamiliesForUser_RemovesOnlyRevokedOrExpiredForThatUser`
  (`internal/repository/refresh_token_test.go:138-174`, verified against
  current source) for structure (active/revoked/expired fixtures,
  `t.Cleanup` per row) — but unlike the per-user precedent, the
  "different user" fixture must also be asserted **removed** here (it's
  revoked, and this method has no user scoping), not asserted to survive.
- Manual verification (documented in this doc, no `.http` file):
  1. `go run main.go` with `APP_ENV=production` and again unset — confirm
     the startup log line matches the expected Gin mode each time.
  2. Start the server, send a slow request (or hold a connection open),
     `kill -TERM <pid>`, and `curl localhost:$PORT/health` during the 10s
     grace window — confirm the in-flight request completes and the new
     request is refused (connection reset), not hung indefinitely.
  3. Temporarily lower the sweep interval locally (not committed), create
     a revoked/expired `refresh_tokens` row via the dev DB, observe it
     removed on the next tick.
