# Audit findings archive (reference for PACK-021 – PACK-028)

> **This is not an implementation-ready handoff doc — it's a shared
> reference archive**, mirroring the `epic-6-findings.md` pattern: raw
> findings captured once, then split into right-sized tickets
> (`PACK-021`–`PACK-028` in `docs/specs/master-spec.md`, Epic 7), each
> covering a numbered subset below. No acceptance criteria are agreed for
> any of them yet — **before writing any of these tickets' own real
> handoff docs, run `/grill-me` for that specific ticket and read the item
> numbers it references here first.**
>
> **Provenance note**: these findings are carried from an external LLM
> audit ("Fable"), given to Claude by the user as-is — not independently
> re-verified line-by-line against current source in this session, the
> way `epic-6-findings.md`'s first three items were (Claude had direct
> repo access and confirmed each before acting). Each ticket's own
> `grill-me` should confirm the specific claim (file/line, current
> behavior) against current source before writing acceptance criteria —
> same discipline as the precedent-verification rule already in
> `~/.claude/CLAUDE.md`, applied to a finding's source instead of a code
> precedent.

## Origin

At the close of PACK-020 (the last ticket of the original backlog), the
user had a separate LLM audit the project — full review of handlers,
repositories, auth, config, middleware, migrations, and tests, plus
`LESSONS.md`, the master spec, and the global process file — asking for:
security vulnerabilities, an idiomatic-Go assessment, and suggestions for
improving the AI-led dev process itself ahead of moving into a frontend
project. That audit found two things Fable itself flagged as skills the
user supposedly already had ("engineering:architecture", "deploy-checklist")
that turned out not to exist anywhere — confirmed fabricated during the
process-improvement work this findings doc is a sibling of. The
process-improvement suggestions were acted on separately (global
`CLAUDE.md` + skill edits, tracked outside this project). **This doc
captures only the code-level findings** — security and Go-idiom — which
become Epic 7.

## Overall assessment (repeated here so it isn't lost)

The audit's security review found a solid baseline: every query
parameterized (no SQL injection), JWT parser pins the signing method (no
alg-confusion), OAuth state is `crypto/rand` and one-time-use, the refresh
cookie is HttpOnly + Secure-in-prod + SameSite=Lax, ownership failures
return 404 not 403 (no existence leak), error responses don't leak
internals, access/refresh tokens use separate secrets, the ID token is
verified via the OIDC verifier rather than trusting the userinfo endpoint,
and the ownership model was checked across all 28 authenticated routes
with no IDOR found. On idiom: consumer-defined repository interfaces
sized to what each handler needs, consistent `%w` error wrapping, black-box
handler tests, `context.Context` as first param throughout, `//go:embed`
for migrations — described as "well above what AI-led development
typically produces." **Nothing below is an emergency** — same framing as
Epic 6's own findings: a "worth doing deliberately" list.

## Findings

### Worth doing soon (production-readiness / security, no live exploit but real gaps)

1. **No HTTP server timeouts.** `server.Run(...)` reportedly uses a
   default `http.Server` — no `ReadHeaderTimeout`/`ReadTimeout`/
   `WriteTimeout`/`IdleTimeout` (slowloris exposure), and no graceful
   shutdown (`signal.NotifyContext` + `Shutdown`). → **PACK-021**

2. **Gin production posture, release-mode half.** `gin.Default()` runs in
   debug mode unless `GIN_MODE=release` is set from `cfg.Environment`. →
   **PACK-021**

3. **No rate limiting or request body size cap.** Write endpoints and
   `/auth/refresh` in particular accept unlimited request rates; `bindJSON`
   will read an arbitrarily large body. → **PACK-022**

4. **Gin production posture, trusted-proxies half.** Gin's default trusts
   all proxies for client-IP derivation (`SetTrustedProxies(nil)` not
   called) — not exploitable today since `ClientIP()` isn't read anywhere,
   but becomes load-bearing the moment rate limiting (item 3) or IP-based
   logging is added. → **PACK-022**

5. **Unbounded in-memory OAuth state map (DoS).** `GenerateState` adds an
   entry on every hit to the unauthenticated `/auth/google/login`;
   expired entries are only swept when that exact state is later
   validated. Looping the login endpoint grows the map without bound; it
   also doesn't survive restarts or horizontal scaling. → **PACK-023**

6. **Deterministic-gates process gap: DB tests can silently skip instead
   of failing loudly.** Distinct from the specific `DATABASE_URL`
   silent-skip incident already caught and process-fixed in
   `LESSONS.md`'s 2026-07-10 (PACK-018) entry — this item is about adding
   an actual CI pipeline (GitHub Actions: `gofmt -l`, `go vet`,
   `golangci-lint`, `govulncheck`, `go test`) as the structural backstop,
   plus making the repository suite fail (not skip) without
   `DATABASE_URL` unless an explicit opt-out flag is passed, so silence is
   never possible rather than just discouraged. → **PACK-024**

7. **No indexes beyond primary keys.** Every FK column and every
   `WHERE user_id = $1` query is a sequential scan. Irrelevant at current
   scale; a one-migration fix. → **PACK-025**

8. **No API contract artifact.** The contract currently lives in handler
   code and `.http` files only. An OpenAPI spec generated from the
   existing handlers would be both the frontend-integration bridge
   (typed client generation, contract-diff instead of debugging-session
   mismatches) and a strong manual-verification artifact for the API
   itself. → **PACK-026**

### Worth knowing about (real, lower urgency, or explicitly deferred)

9. **Refresh tokens can't be revoked.** Known, documented trade-off (no
   server-side store; logout only clears the cookie). Acceptable for a
   dev-only API; before a real frontend ships, add rotation with reuse
   detection (new refresh token issued on every `/auth/refresh`, a
   token-family ID, kill the family on reuse of a stale token). **Deferred
   explicitly** — pick up alongside the future frontend auth-integration
   ticket, not standalone. → **PACK-027**

10. **`email_verified` not checked** on the Google ID token claims. Low
    collision risk since users are keyed on `google_id` not email, but a
    user row can still be created with an unverified email. → **PACK-028**

11. **Refresh flow turns bad input into 500s.** `claims.Subject` passed
    straight into `GetUserByID` — a non-UUID subject produces a Postgres
    syntax error → 500 instead of 401. Related: nothing asserts at
    startup that the two JWT secrets are distinct. → **PACK-028**

12. **Ownership checks are read-then-act, not enforced in the query
    itself.** Handlers fetch, check ownership, then mutate
    `WHERE id = $1` without `user_id` in the clause. Not exploitable
    today (ownership never changes mid-flight) — free defense-in-depth,
    not a live bug. → **PACK-028**

13. **Misc smaller items**: `GoogleCallback`/`RefreshToken` use
    `context.Background()` instead of `c.Request.Context()` (client
    disconnect doesn't cancel in-flight work); `ILIKE` search doesn't
    escape `%`/`_`; `gen_token.go` only refuses to run when
    `APP_ENV=production` is explicitly set, not guarded on DB host;
    confirm the Neon `DATABASE_URL` includes `sslmode=require` (`lib/pq`
    won't force it). → **PACK-028**

14. **`BulkAddItems` does N single-row inserts in a loop,
    non-transactionally.** A mid-loop failure leaves a partially-added
    batch; also N round-trips instead of one `INSERT ... SELECT`. →
    **PACK-028**

15. **No structured logging** — `fmt.Println` in `db.go`, Gin's default
    logger elsewhere. **This reopens an Epic-6 dismissal — see below.** →
    **PACK-028**

16. **`main()` does everything.** Extracting route registration into
    `func newRouter(deps...) *gin.Engine` shrinks `main` to config/wiring/
    run and makes a true end-to-end test (real router, mocked repos)
    possible. Cosmetic at this size otherwise. → **PACK-028**

17. **Naming stutter/inconsistency**: `repository.CategoryRepository`
    stutters (stdlib style: `http.Client`, not `http.HTTPClient`); only
    `user`'s constructor gets the `Postgres` prefix
    (`NewPostgresUserRepository`) while every other repo doesn't
    (`NewCategoryRepository`). → **PACK-028**

18. **`eventDate` as `*string` rather than `time.Time`/a date type** —
    pragmatic for JSON, but pushes formatting/validation into every
    consumer. Minor. → **PACK-028**

### Awareness-only (no ticket — recorded so it isn't re-litigated as a fresh finding later)

- **`nil, nil` for not-found vs. a sentinel error.** The mainstream Go
  idiom is a sentinel (`var ErrNotFound = errors.New(...)` +
  `errors.Is`) — `nil, nil` means every caller must remember the nil
  check. This codebase's `nil, nil` convention (established and, per
  PACK-016, made *consistent*) is "the road less traveled" but applied
  uniformly, which matters more than which convention was picked. Not
  changing.
- **`lib/pq` is in maintenance mode; `pgx` is the actively-developed
  standard.** Fine to defer until something (a feature `lib/pq` can't
  support) forces the migration.

### Explicitly reviewed and dismissed, carried forward from Epic 6 (still holds, not reopened)

- **Global `var DB *sql.DB`** — Epic 6's dismissal reasoning still holds:
  every repository already takes `*sql.DB` as an explicit constructor
  parameter, and `main.go` is the only actual consumer of the global.
  This audit re-flagged it; the calculus hasn't changed, so it stays
  dismissed. Not re-adding a ticket for it.

### Explicitly reopened from Epic 6 (not a silent reversal — reason stated)

- **`fmt.Println` logging in `db.go`.** Epic 6 dismissed this as "fine
  until a proper logger... gets introduced project-wide." Reopening it
  here specifically: PACK-021 (server timeouts/graceful shutdown) and
  PACK-022 (rate limiting) both add behavior that's only debuggable in
  production with real structured logging — a rejected request or a slow
  shutdown needs to be correlatable to a request context, which
  `fmt.Println` can't do. The condition Epic 6's dismissal was waiting on
  ("until... gets introduced project-wide") is now imminent because of
  this same epic's other tickets, not because the original judgment was
  wrong. Bundled into item 15 / **PACK-028**.

## Ticket mapping

- **PACK-021** — items 1-2 (server lifecycle: timeouts, graceful shutdown,
  release mode)
- **PACK-022** — items 3-4 (request-level abuse protection: rate
  limiting, body size cap, trusted proxies)
- **PACK-023** — item 5 (OAuth state store fix)
- **PACK-024** — item 6 (CI pipeline + fail-loud DB tests)
- **PACK-025** — item 7 (DB indexes migration)
- **PACK-026** — item 8 (OpenAPI spec)
- **PACK-027** — item 9 (refresh token rotation — deferred, not
  standalone)
- **PACK-028** — items 10-18 (minor security/idiom cleanup, bundled)

Pick any of these up with its own `/grill-me` when ready; each is scoped
enough in `master-spec.md` to start from, but still needs its own
interview, source verification per the provenance note above, and real
handoff doc before implementation.
