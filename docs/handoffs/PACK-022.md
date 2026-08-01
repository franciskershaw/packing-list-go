# PACK-022 — Request-level abuse protection

## Context

Source: `docs/handoffs/audit-2026-07-11-findings.md` items 3-4 (no rate
limiting or request body size cap on write endpoints/`/auth/refresh`; Gin's
default trusts all proxies for client-IP derivation, which becomes
load-bearing the moment rate limiting reads `ClientIP()`). Both findings
are addressed together in this ticket since they're coupled — a rate
limiter keyed on `ClientIP()` is meaningless without a correct trusted-
proxies setup behind it.

Design gate: rate limiting is a new pattern for this codebase (no existing
precedent), with real blast radius (every route). Decided against an ADR
— small and self-contained enough for this handoff doc to carry the
decision trail, matching PACK-021's precedent for the same call.

Key decisions from interview:

- **Scope**: broadened from `master-spec.md`'s original `/auth/*`-only
  framing after discussion — a lenient **global default** across the
  whole API, plus a **stricter override on `/auth/*`**. Applying it
  broadly costs no extra design complexity (same middleware, different
  `Rate` per route group), so there's no reason to leave the rest of the
  API unprotected.
- **Library**: `github.com/ulule/limiter/v3` (in-memory store via
  `drivers/store/memory`, Gin adapter via `drivers/middleware/gin`) rather
  than a hand-rolled `map[string]*rate.Limiter`. Avoids reinventing
  bounded-eviction logic, and its store abstraction means swapping to the
  Redis store later (if this ever scales to multiple containers behind a
  load balancer) is a config change to the same library, not a rewrite —
  not built now, just noted as the upgrade path. In-memory is correct for
  the current single-droplet/single-container deployment plan; a
  multi-instance deployment would silently multiply the effective limit
  by instance count since state isn't shared — that's the revisit-when
  trigger, not something to guard against now.
- **Limits**: global `Rate{Period: time.Minute, Limit: 60}` per IP;
  `/auth/*` group `Rate{Period: time.Minute, Limit: 10}` per IP (its own
  independent limiter instance/store — hitting the `/auth/*` limit doesn't
  consume the global bucket or vice versa; a request under both is
  checked against both). `ulule/limiter`'s sliding-window algorithm
  smooths the classic fixed-window edge-burst problem, so no separate
  burst parameter is needed beyond `Limit` itself. Verify the exact
  `Rate`/store/middleware constructor API against the library's current
  source/docs at implementation time — not assumed from memory here.
- **Auth routes need a route group**: currently registered as flat
  `server.GET("/auth/google/login", ...)` etc. — these move under
  `auth := server.Group("/auth")` (relative paths, e.g.
  `auth.GET("/google/login", ...)`) so the stricter limiter can be
  `.Use()`'d on the group, mirroring the existing `authed :=
  server.Group("/"); authed.Use(...)` pattern for authenticated routes.
- **Body size cap**: 1 MB, applied globally. No file/image uploads exist
  anywhere in this API (categories/items/templates/lists are all small
  JSON payloads, including the largest bulk-delta requests), so 1 MB is
  generous headroom, not a tight fit. Implementation approach: wrap
  `c.Request.Body` in `http.MaxBytesReader`, read it fully inside the
  middleware (bounded by the cap), and re-set `c.Request.Body` to a fresh
  reader over the already-read bytes so downstream `ShouldBindJSON` still
  works normally — catching the too-large condition in the middleware
  itself (clean 413 with our own response shape) rather than letting it
  surface later as a raw `http.MaxBytesReader` error string inside a
  handler's own 400 response.
- **Trusted proxies**: not hardcoded to `nil`. New `cfg.TrustedProxies
  []string` config value (comma-separated env var, parsed the same way
  `cfg.Environment` etc. already are in `config/config.go`), defaulting to
  empty/`nil` when unset (matches today's safe local-dev behavior — no
  proxy in front, so `RemoteAddr` is trusted directly). `main.go` calls
  `server.SetTrustedProxies(cfg.TrustedProxies)` instead of a hardcoded
  `nil`. The actual production value (Cloudflare's published IP ranges,
  or a local reverse-proxy hop if one gets added to the Docker setup) is
  a deployment-config decision made once the droplet topology is final —
  explicitly out of this ticket's scope.
- **Response shape**: every existing error response in this codebase uses
  `gin.H{"error": "..."}` (verified across `auth_handler.go`,
  `category_handler.go`, etc., no exceptions found). `ulule/limiter`'s Gin
  middleware takes a custom `ErrorHandler` option — used to emit
  `gin.H{"error": "rate limit exceeded"}` on 429 (with a `Retry-After`
  header set) instead of the library's own default format. Body-limit
  middleware emits `gin.H{"error": "request body too large"}` on 413.
  Both match the existing convention exactly.
- **File placement**: `internal/middleware/rate_limit.go` and
  `internal/middleware/body_limit.go`, following the existing one-file-
  per-concern split (`auth.go`, `error_logger.go`) — no new pattern.

## Acceptance criteria

- [x] AC1 — `middleware.RateLimit(store limiter.Store, rate limiter.Rate)
      gin.HandlerFunc` wrapping the `ulule/limiter` Gin adapter returns 429
      with `gin.H{"error": "rate limit exceeded"}` and a `Retry-After`
      header once a per-IP limit is exceeded; requests under the limit
      pass through unchanged.
      **Found during implementation, bundled in**: the library's default
      `OnError` handler (genuine store failures, distinct from limit-
      reached) is `panic(err)` — recovered by Gin's `Recovery()` middleware
      so it doesn't crash the server, but the response wouldn't match this
      project's `gin.H{"error": ...}` convention and could leak a stack
      trace in debug mode. Added a `WithErrorHandler` override returning a
      clean `500 gin.H{"error": "internal server error"}`, covered by
      `TestRateLimit_StoreErrorReturnsCleanServerError` (a small in-file
      `failingStore` fake implementing `limiter.Store`). Same shape as
      this ticket's own body-cap/trusted-proxies pairing — a directly
      adjacent gap in code this AC already touches, not a new feature.
- [ ] AC2 — Global rate limiter (`Rate{Period: time.Minute, Limit: 60}`)
      applied via `server.Use(...)` covering the whole API.
- [ ] AC3 — `/auth/*` routes regrouped under `server.Group("/auth")` with
      their own stricter rate limiter (`Rate{Period: time.Minute, Limit:
      10}`) applied via `.Use()` on that group, independent of the global
      limiter.
- [ ] AC4 — `middleware.BodyLimit(maxBytes int64) gin.HandlerFunc` returns
      413 with `gin.H{"error": "request body too large"}` for any request
      body exceeding 1 MB; requests under the cap are unaffected and still
      bind normally downstream. Applied globally via `server.Use(...)`.
- [ ] AC5 — `config.Config` gains `TrustedProxies []string`, parsed from a
      comma-separated env var, defaulting to `nil`/empty when unset.
      `main.go`'s `server.SetTrustedProxies(...)` reads from
      `cfg.TrustedProxies` instead of a hardcoded `nil`.

## Non-goals

- Redis-backed store / any change for horizontal scaling — noted as a
  future upgrade path via the same library, not built now.
- Hardcoding Cloudflare's IP ranges (or any concrete trusted-proxies
  value) — that's a deployment-time config decision, not part of this
  ticket's code.
- Per-user or per-route-beyond-`/auth/*` rate limiting granularity — the
  global default covers the rest of the authenticated API; a more
  granular scheme is a future ticket if it's ever needed.
- CAPTCHA, account lockout, or any other anti-abuse layer beyond rate
  limiting + body cap + trusted proxies.
- Infrastructure-level rate limiting/DDoS protection (Cloudflare, a
  reverse proxy's own `limit_req`) — already planned as part of the
  deployment topology, not tracked as a backlog item per this
  conversation.
- ADR — explicitly decided against (see Context).
- New `requests/*.http` file — this ticket adds no new HTTP endpoint or
  request/response contract, only cross-cutting middleware behavior on
  existing endpoints, same justification PACK-021 used to skip one. Manual
  verification below is a documented curl sequence instead.

## Expected test files

- `internal/middleware/rate_limit_test.go` (mounts the middleware alone on
  a fresh `gin.New()`, per this project's handler-test convention):
  - `TestRateLimit_AllowsRequestsUnderLimit`
  - `TestRateLimit_BlocksRequestsOverLimit` — asserts 429, the
    `gin.H{"error": "rate limit exceeded"}` body, and a `Retry-After`
    header present.
  - `TestRateLimit_TracksDifferentIPsSeparately` — two distinct
    `RemoteAddr`s each get their own bucket.
  - `TestRateLimit_StoreErrorReturnsCleanServerError` — added mid-
    implementation, see AC1 note above.
- `internal/middleware/body_limit_test.go`:
  - `TestBodyLimit_AllowsRequestsUnderCap`
  - `TestBodyLimit_BlocksRequestsOverCap` — asserts 413 and the
    `gin.H{"error": "request body too large"}` body; confirms the handler
    itself is never reached (e.g. via a spy handler asserting it wasn't
    called).
- `config/config_test.go` (add to the existing file, mirroring
  `TestLoad_EnvironmentDefaultsToDevelopment`/`TestLoad_EnvironmentReadsAppEnv`
  structure, verified against current source):
  - `TestLoad_TrustedProxiesDefaultsToEmpty`
  - `TestLoad_TrustedProxiesParsesCommaSeparatedList`
- No new `main_test.go` entries — wiring `server.Use(...)`,
  `server.Group("/auth")`, and `server.SetTrustedProxies(cfg.TrustedProxies)`
  is pure composition of already-tested pieces, consistent with how the
  existing `AuthMiddleware` wiring isn't separately unit-tested in
  `main_test.go` either.
- Manual verification (documented here, no `.http` file — see Non-goals):
  1. `go run main.go`, then `for i in $(seq 1 15); do curl -i
     http://localhost:$PORT/health; done` — confirm the first 60 requests
     (global limit) succeed and it never trips within just 15 hits;
     repeat with a tighter temporary local limit (not committed) to
     confirm the 429 path actually fires and includes `Retry-After`.
  2. Same loop against `POST /auth/refresh` (or `/auth/google/login`) —
     confirm it 429s well before 60 hits, at the `/auth/*` group's
     stricter limit.
  3. `curl -X POST http://localhost:$PORT/categories -H "Authorization:
     Bearer $DEV_TOKEN" -H "Content-Type: application/json" --data-binary
     @<(head -c 2000000 /dev/urandom | base64)` — confirm 413 with the
     expected error body; confirm a normal-sized request still succeeds.
  4. Confirm `SetTrustedProxies(nil)` behavior is unchanged for local dev
     (no proxy in front) — `c.ClientIP()` still resolves to the real
     local request origin, not broken by the config plumbing change.
