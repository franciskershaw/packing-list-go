# PACK-038 — Docker/deployment target

## Context

Split out of PACK-024's grill-me (2026-08-01) — that ticket covered CI
(gofmt/vet/golangci-lint/govulncheck/test) only; no Dockerfile or
deployment config exists anywhere in this repo. `docs/specs/master-spec.md`
deferred the hosting-target decision to this ticket's own `grill-me`. That
grill-me happened 2026-08-02.

**Decided direction**: DigitalOcean, via GitHub Actions, reusing the
developer's existing shared Droplet (already running `events-api` and
`salary-split-api`) rather than DO App Platform or a new dedicated
Droplet.

**Design gate**: this is the first hosting/deployment decision for
`packing-list-go`, but it follows an established external precedent —
`../events-api/.github/workflows/deploy.yml` and
`../salary-split/.github/workflows/deploy-api.yml` (near-identical to each
other), backed by `../mern-deployment/`'s `droplet.md` /
`manualBackend.md` / `automated-deployment.md` playbooks. No ADR — declined
at grill-me (same reasoning as PACK-024: reversible, conventional choices,
not the auth/data-model class of decision the ADR process exists for).
This Context section is the record.

Key decisions from the interview:

- **Hosting: reuse the existing Droplet, not DO App Platform.**
  App Platform was seriously considered (the user explicitly raised
  "is there a simpler way") but rejected on cost structure: App Platform
  bills per service, while a Droplet is one flat ~£5/mo box the developer
  is already using to consolidate several low-traffic side projects. Adding
  a third app to the existing Droplet is marginally free; App Platform
  would add its own recurring bill. Revisit only if this or another
  Droplet app's traffic ever grows enough to justify isolating it onto
  managed infrastructure.
- **Domain: `api.packitapp.co.uk`.** The developer already owns
  `packitapp.co.uk`, currently still on the registrar's default
  nameservers (not yet on Cloudflare) — so the full
  `automated-deployment.md` Cloudflare setup flow is needed (add site, A
  record for `api` → Droplet IP, switch nameservers, wait for propagation,
  set SSL/TLS mode to Full (Strict)). Only the `api` subdomain is being
  configured now; root/`www` records are deferred to whatever ticket picks
  a frontend deployment target (`packing-list-react`'s master-spec already
  flags that as a revisit-later item).
- **Neon IP allowlisting**: unknown at grill-me time — the developer will
  check the Neon dashboard before first deploy. Unlike this repo's Node
  reference projects (Mongo Atlas connection strings show no IP-allowlist
  step), Neon's IP Allow feature, if enabled, would need the Droplet's
  outbound IP added or the deploy would fail with what looks like a
  generic connection timeout rather than an obvious auth error.
- **Separate production database.** PACK-024's handoff doc deliberately
  named the CI secret `DEV_DATABASE_URL` (not `DATABASE_URL`) "so the name
  `DATABASE_URL` (or `PROD_DATABASE_URL`) stays free at the GitHub-secrets
  level for whatever deploy workflow ends up needing it, if a separate prod
  DB is required later." That's this ticket: a new Neon branch
  (`production`) gets its own connection string, stored as a new
  `PROD_DATABASE_URL` GitHub secret, keeping real trip data isolated from
  whatever CI/local dev churns through on the existing branch. Neon
  migrations run automatically at app startup (`db/db.go`'s
  `migrate.NewWithSourceInstance`/`m.Up()`), so the new branch's schema
  is self-provisioning — but `db/seeds/categories.sql` is **not** run
  automatically anywhere in the app (confirmed: no reference to it outside
  `db/seeds/`), so it needs a one-time manual run against the new branch
  before item-creation works at all (same gap PACK-033 fixed for the dev
  DB).
- **Dockerfile: multi-stage, `distroless/static` final image.** Go's
  deployment story differs from every Node reference project (no runtime
  package manager step — `CGO_ENABLED=0 GOOS=linux go build` produces a
  single static binary). Chosen over `alpine` for minimal attack
  surface/image size. Trade-off accepted: distroless has no shell and no
  `curl`, so the reference deploy scripts' `docker run --health-cmd="curl
  ..."` flag (an internal Docker-level healthcheck, cosmetic — only
  surfaces in `docker ps`/`docker inspect`) is dropped rather than ported.
  The deploy workflow's actual safety gate is unaffected: it's the
  separate SSH "Health Check" step that curls the app **from the Droplet
  host** (outside the container, against the mapped port), which does not
  require anything inside the image. That step targets this repo's
  existing `GET /health` endpoint (`main.go:88`) — the Node references
  curl their bare root path since they have no dedicated health route.
- **Deploy strategy: keep the full blue-green swap, but fix a real
  rollback gap the reference repos never actually closed.** Matching
  `events-api`/`salary-split-api`'s shape (tag current `:latest` as
  `:previous` before pushing, run the new container as `<name>-new`,
  host-side curl health-poll, then stop/remove the old container and
  rename `-new` into place). Considered simplifying to a plain
  stop/pull/restart given this is a low-traffic personal app, but decided
  to keep parity with the other two repos' `deploy.yml` — consistency
  across all three makes the next side project's setup a copy-paste, and
  the extra script lines cost nothing ongoing.
  - **Gap found while reviewing this doc**: in both reference `deploy.yml`
    files, the "Deploy Docker Container" step stops and removes the
    *real-named* container **before** the new one's health is ever
    checked (`docker stop ${{ env.CONTAINER_NAME }}
    ${{ env.CONTAINER_NAME }}-new || true` — both containers bind the same
    fixed host port, so killing old is how the port gets freed for
    `-new`). If the later "Health Check" step's curl-poll loop times out,
    the script just `exit 1`s — nothing restarts the old image. The
    `:previous` Docker Hub tag gets pushed during the build job but is
    never pulled or run by either deploy.yml; it's a manually-usable
    rescue tag, not an automated recovery path. Net effect: a failed
    health check today means the old container is already gone, the new
    one is unhealthy, and the site is down until someone SSHes in and
    manually restarts `:previous` by hand.
  - **Fix for this repo's `deploy.yml`**: the "Health Check" step's
    failure branch (currently just `docker logs ...-new; exit 1`) also
    removes the failed `-new` container, pulls `:previous`, and runs it
    back under the real container name before exiting 1 — so a failed
    deploy is left in a known-good state automatically, not just flagged
    as broken. Scoped to this repo's own `deploy.yml` only; not
    backporting the fix to `events-api`/`salary-split-api` as part of this
    ticket.
- **Deploy gated on CI passing**, not triggered independently. Unlike the
  Node reference repos (which predate this repo's CI investment and
  trigger deploy on push to `main` with no relationship to any lint/test
  gate), `packing-list-go` already has a real `ci.yml` from PACK-024. The
  existing `checks` job gets a new `build-and-deploy` job added alongside
  it in the same workflow file, with `needs: checks` and
  `if: github.ref == 'refs/heads/main' && github.event_name == 'push'` —
  a lint failure or `govulncheck` finding blocks deployment automatically,
  rather than deploying in parallel regardless of CI's outcome.
- **Cookie/CORS scope, resolved after some back-and-forth at grill-me**:
  the actual bar for this ticket is reaching `https://api.packitapp.co.uk/health`
  in a browser — not a fully working production frontend integration
  (`packing-list-react`'s frontend deployment target is still undecided,
  likely Vercel later). But the developer wants Google Cloud Console
  registered correctly now, "to give ourselves the best possible chance
  of [the Vercel frontend] working out of the box" once that's built,
  accepting something may still need fixing then. Net scope: the Google
  Console redirect URI gets registered now, and the code changes below are
  made now (config-driven, inert until `FRONTEND_URL` points somewhere
  real) — but wiring an actual production frontend integration is **not**
  a goal of this ticket.
  - `internal/handler/auth_handler.go:66-77`'s `setRefreshCookie` — used
    for issuing (`GoogleCallback`, line 172), rotating (`RefreshToken`,
    line 242), and clearing (`Logout`, line 264) the refresh cookie — sets
    `SameSite=Lax` unconditionally today. Its own comment (lines 68-73)
    already documents why that was correct for PACK-032's setup ("the
    frontend proxies API calls through its own origin in local dev... Lax
    always permits [the OAuth redirect]... Revisit if a cross-origin
    production deployment changes that assumption") — this ticket is that
    revisit. Fix: `SameSite=None` when `cfg.Environment == "production"`,
    `Lax` otherwise. **Only the refresh cookie** — `setOAuthStateCookie`/
    `clearOAuthStateCookie` (lines 79-88) stay `Lax` unchanged, since
    `oauthState` is only ever round-tripped within `api.packitapp.co.uk`
    itself (set on `/auth/google/login`, read back on
    `/auth/google/callback` after Google's own top-level-navigation
    redirect back to the same host) and is never actually cross-site.
  - New CORS middleware, added to `internal/middleware/` (own file + own
    test file, matching the existing hand-rolled pattern in
    `body_limit.go`/`error_logger.go`/`rate_limit.go` rather than pulling
    in `gin-contrib/cors`): allows only `cfg.FrontendURL` as the origin,
    with credentials, registered in `main.go` alongside the other global
    middleware (`main.go:83-85`).
  - Prod `FRONTEND_URL` secret is set to `http://localhost:5173` for
    now — not a real prod value, but the only concrete origin that exists
    today, and it lets the manual-verification step below actually
    exercise the full OAuth round-trip (Console config + cookie + CORS)
    against a local dev frontend rather than just eyeballing the Console
    setup. Swap it to the real Vercel origin once that's decided — flagged
    as a follow-up in `packing-list-react`'s master-spec already, not
    re-flagged here as a new item.

## Acceptance criteria

- [ ] `Dockerfile` added: multi-stage build, `golang:1.26-alpine` (or
      whatever tag matches `go.mod`'s current toolchain directive) builder
      stage producing a `CGO_ENABLED=0 GOOS=linux` static binary, final
      stage `gcr.io/distroless/static`. `.dockerignore` added alongside it.
- [ ] `internal/handler/auth_handler.go`'s `setRefreshCookie` sets
      `SameSite=None` when `cfg.Environment == "production"`, `Lax`
      otherwise; comment at lines 68-73 updated to reflect the new
      behavior instead of describing the now-superseded assumption.
      `oauthState` cookie functions unchanged.
- [ ] New CORS middleware (`internal/middleware/cors.go` + test file)
      allowing only `cfg.FrontendURL` as origin, with credentials;
      registered in `main.go`.
- [ ] `.github/workflows/ci.yml` gains a `build-and-deploy` job
      (`needs: checks`, `if: github.ref == 'refs/heads/main' &&
      github.event_name == 'push'`) that: logs into Docker Hub, tags
      current `:latest` as `:previous`, builds and pushes the image
      (`docker/build-push-action`, `linux/amd64`, GHA layer caching),
      then SSHs to the Droplet to install/verify the `api.packitapp.co.uk`
      nginx server block + certbot cert (idempotent — skip if SSL config
      already present), and performs the blue-green container swap
      (`<name>-new` → host-side curl poll of `/health` → stop/remove old →
      rename) mirroring `events-api`/`salary-split-api`'s `deploy.yml`,
      minus the in-container `--health-cmd` flag (no shell/curl in the
      distroless image), **plus** an automated rollback: on health-poll
      timeout, remove the failed `-new` container, pull `:previous`, and
      run it back under the real container name before exiting 1 — a gap
      neither reference repo's `deploy.yml` actually closes today (see
      Context).
- [ ] `nginx/packing-list-api.conf` added to the repo (matching
      `events-api/nginx/*.conf`'s shape), proxying `api.packitapp.co.uk`
      to `localhost:<chosen-port>` — port chosen to avoid colliding with
      the Droplet's existing `:5300` (salary-split-api) and `:5500`
      (events-api).
- [ ] All required GitHub repo secrets added (see manual steps below) —
      `DO_HOST`, `DO_USERNAME`, `DO_SSH_KEY`, `DOCKER_USERNAME`,
      `DOCKER_PASSWORD`, `CERTBOT_EMAIL`, `PROD_DATABASE_URL`,
      `JWT_SECRET_ACCESS`, `JWT_SECRET_REFRESH`, `JWT_SECRET_OAUTH_STATE`,
      `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`,
      `FRONTEND_URL`.
- [ ] `https://api.packitapp.co.uk/health` reachable and returns 200 in a
      real browser, with a valid (non-self-signed) certificate.
- [ ] Manual OAuth round-trip verified once: visiting
      `https://api.packitapp.co.uk/auth/google/login` in a browser
      completes Google's consent screen and lands back on
      `http://localhost:5173/auth/callback` with a working session
      (confirms Console redirect URI, `SameSite=None` refresh cookie, and
      CORS are all correctly wired together) — run with the local frontend
      dev server up.

## Non-goals

- A production frontend deployment target — `packing-list-react`'s
  deployment target stays "undecided," per its own master-spec. This
  ticket only makes the backend reachable and OAuth-ready for whenever
  that lands.
- Root/`www` DNS records or any frontend-hosting Cloudflare config — only
  the `api` subdomain is touched.
- IaC (Terraform/Pulumi/etc.) — raised and explicitly declined at
  grill-me; the manual-Droplet pattern is already proven across two other
  repos and the developer hasn't used IaC before. Not revisited unless a
  future ticket's scope specifically calls for it.
- A dev-database-vs-prod-database migration/backfill of existing data —
  the new `production` Neon branch starts fresh; there's no existing
  production data to migrate.
- Changing `PACK-028`'s still-open "JWT-secrets-distinct startup
  assertion" item — this ticket generates genuinely distinct prod JWT
  secrets as a manual step (not reused from `.env`), but does not add
  code to *enforce* distinctness at startup. That stays PACK-028's scope.
- `PACK-037` (bulk packed-toggle batching) and any other still-open
  ticket — unrelated, not touched here.

## Expected test files

This ticket is infra-heavy (Dockerfile, GitHub Actions YAML, nginx conf) —
most of it has no meaningful Go-testable behavior. The one real behavioral
surface is the cookie/CORS change:

- `internal/handler/auth_handler_test.go`:
  - `TestGoogleCallback_HappyPath_SecureCookieInProduction` (currently
    line 222-264, precedent for this file's existing
    `testConfig("production")` pattern) — assertion at line 264 changes
    from `assert.Equal(t, http.SameSiteLaxMode, refreshCookie.SameSite)`
    to `http.SameSiteNoneMode`.
  - New test: `TestLogout_ClearsCookie_SameSiteNoneInProduction` — mirrors
    the existing `TestLogout_ClearsCookie` (line 863, currently
    `testConfig("development")` only) but with `testConfig("production")`,
    asserting the cleared cookie is also `SameSiteNoneMode`. The existing
    `TestLogout_ClearsCookie` stays as-is (asserts `Lax` under
    `"development"`, still correct — unchanged).
  - `TestLoginWithGoogle_SetsOAuthStateCookie` (line 434) — unchanged, no
    new assertion needed; confirms `oauthState` stays `Lax` regardless of
    environment (already implicitly covered by not varying environment in
    that test; add an explicit `testConfig("production")` variant only if
    review flags the implicit coverage as too indirect).
- `internal/middleware/cors_test.go` (new) — asserts: request from the
  configured `FRONTEND_URL` origin gets `Access-Control-Allow-Origin` +
  `Access-Control-Allow-Credentials: true`; request from a different
  origin does not; `OPTIONS` preflight handled correctly. Follow
  `body_limit_test.go`'s existing structure/conventions for this package.
- Manual verification (no `.http` file — this ticket has no new JSON
  endpoints): the acceptance criteria's browser-based `/health` check and
  OAuth round-trip above, plus the existing `requests/*.http` files
  re-run once against `https://api.packitapp.co.uk` (using
  `scripts/gen_token.go`'s existing dev-token approach, same as local) to
  confirm already-existing endpoints behave identically in production.

## Manual steps (not code — tracked here since this ticket is mostly ops)

1. **Neon**: check whether IP Allow is enabled on the current project; if
   so, add the Droplet's IP once known. Create a new `production` branch;
   copy its connection string.
2. **Neon (production branch)**: after first deploy (migrations
   auto-apply on startup), run `db/seeds/categories.sql` against the new
   `production` branch via **Neon's web SQL Editor** — not `psql`, which
   isn't installed locally (established default for ad hoc SQL on this
   project since PACK-033, see `LESSONS.md` 2026-07-26). Confirm the
   result is exactly 11 category rows (the seed's own idempotency guard
   from PACK-033 makes a second accidental run harmless, but checking the
   count once is a cheap sanity check on a branch that's never been seeded
   before). Item creation has no categories to attach to until this runs —
   same gap PACK-033 fixed for the dev branch.
3. **Cloudflare**: add `packitapp.co.uk` as a site (free tier), add an A
   record `api` → Droplet IP, switch the domain's nameservers at the
   registrar to Cloudflare's, wait for propagation, set SSL/TLS mode to
   Full (Strict).
4. **Google Cloud Console**: add `https://api.packitapp.co.uk/auth/google/callback`
   as an authorized redirect URI on the existing OAuth client.
5. **Droplet**: confirm a free port (avoiding the existing `:5300`/
   `:5500`) for `packing-list-api`'s container.
6. **Docker Hub**: confirm/create an access token for `DOCKER_PASSWORD` if
   the existing one used by the other two repos isn't being reused.
7. **GitHub repo secrets** (`packing-list-go` settings — these do not
   carry over from `events-api`/`salary-split-api` even though some values
   are shared, e.g. `DO_HOST`/`DOCKER_USERNAME`): `DO_HOST`, `DO_USERNAME`,
   `DO_SSH_KEY`, `DOCKER_USERNAME`, `DOCKER_PASSWORD`, `CERTBOT_EMAIL`,
   `PROD_DATABASE_URL`, freshly-generated (not reused from `.env`)
   `JWT_SECRET_ACCESS`/`JWT_SECRET_REFRESH`/`JWT_SECRET_OAUTH_STATE`,
   `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
   `GOOGLE_REDIRECT_URI=https://api.packitapp.co.uk/auth/google/callback`,
   `FRONTEND_URL=http://localhost:5173`.
8. **First push to `main`** triggers `build-and-deploy`. Expect real
   debugging on the first run (consistent with PACK-024's own experience —
   "every gate failed for a real reason at least once").

## Close-out

(Filled in at `/close-out` once this ticket ships.)
