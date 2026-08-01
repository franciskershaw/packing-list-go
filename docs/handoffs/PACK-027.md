# PACK-027 — Refresh token rotation with reuse detection

## Context

Filed from the 2026-07-11 audit (`docs/handoffs/audit-2026-07-11-findings.md`,
item 9): refresh tokens today are a single stateless JWT, valid for its full
7-day life with no way to revoke it server-side — `Logout`
(`internal/handler/auth_handler.go`) only clears the browser cookie; a stolen
cookie keeps working until it naturally expires. Deliberately deferred at
filing time ("pick up alongside the future frontend auth-integration
ticket, not standalone") — that condition is now satisfied:
`packing-list-react`'s **PACKFE-002** (Google sign-in & session restore) is
Done (`packing-list-react/docs/specs/auth.md`), and its `client.ts` already
calls `POST /auth/refresh` on every session bootstrap and on any 401.

**Design-gate finding**: new pattern — the first DB-backed, stateful piece
of this codebase's auth model (new table, new repository, a revocation
concept), versus everything else here being stateless JWT. Discussed
whether that warrants a formal ADR: **declined** — this repo has never had
a `docs/adr/` folder, and the existing precedent for a decision of this
kind is `docs/specs/master-spec.md`'s "Key architecture decisions" section,
which already carries the current Auth bullet pointing at
`docs/handoffs/PACK-005.md`. That bullet should be updated at this ticket's
close-out to mention rotation/reuse-detection, pointing here instead of (or
alongside) PACK-005 — not done now, since the decision trail belongs in a
finished ticket's doc, not a not-yet-implemented one.

**Key decisions from the interview:**

- **Family model: one row per login, overwritten in place on each
  rotation** — not an append-only chain. New table `refresh_tokens`
  (migration `000003_refresh_tokens`): `id` (uuid, the family identifier),
  `user_id`, `token_hash`, `previous_token_hash` (nullable),
  `previous_token_rotated_at` (nullable), `expires_at`, `revoked_at`
  (nullable), `created_at`. Raw tokens are never stored — SHA-256 (Go
  stdlib `crypto/sha256`) of the token string, since the token is already
  high-entropy (unlike a user-chosen secret, no need for a slow/salted
  hash).
- **Family ID *is* embedded in the JWT (`familyId` claim) — reversed
  mid-implementation.** Originally decided against this (hash-only lookup,
  to avoid a second source of truth). Manual verification against the real
  server caught why that's insufficient: with only `token_hash`/
  `previous_token_hash` stored, a token more than one rotation stale
  matches neither, `FindFamilyByHash` returns nothing, and
  `RevokeFamily` never gets called — reuse of a 2+-generations-stale token
  silently 401s while the live family (and its actual current cookie)
  stays completely valid. That's a materially weaker guarantee than "reuse
  of a stale token kills the family," the literal phrase from the audit
  finding this ticket implements. Fix: `internal/auth/jwt.go` gained a
  `RefreshClaims{FamilyID string; jwt.RegisteredClaims}` type;
  `GenerateRefreshToken`/`ValidateRefreshToken` now take/return it. The
  repository's `FindFamilyByHash` was replaced with `FindFamilyByID(ctx,
  id, userID)`, so a family is always found by its id regardless of how
  stale the presented hash is — the hash is only used afterward, in Go, to
  decide rotate-vs-revoke. Confirmed against the live server: replaying a
  2-generations-stale token now correctly revokes the family and a
  previously-valid current cookie is rejected immediately after.
- **Grace window for concurrent legitimate refreshes: 10 seconds.** The
  frontend's `client.ts` already de-dupes concurrent refresh calls
  *within* a tab (`refreshPromise` singleton), but nothing coordinates
  *across* tabs — two tabs refreshing near-simultaneously would otherwise
  present the same pre-rotation token to two requests, and naive "any
  mismatch = theft" would kill the family on ordinary multi-tab use. Fix:
  a token matching `previous_token_hash` within 10s of
  `previous_token_rotated_at` is treated as a benign race, not reuse.
  Chosen as generous enough for any realistic same-machine race, tight
  enough not to be a meaningful exploit window.
- **Rotation and the grace-window case are the same code path.** A token
  matching either `token_hash` (normal case) or `previous_token_hash`
  within the grace window (race case) both result in an ordinary rotation
  (new token issued, family's `token_hash`/`previous_token_hash` shifted
  forward again) — there's no way to "reissue the exact same current
  token" to a second racing request, since only its hash is stored
  server-side, and there's no need to: allowing a second legitimate
  rotation through is equivalent in effect and much simpler than trying to
  recover the original raw value.
- **Reuse detected → revoke, not just reject.** A token matching
  `previous_token_hash` *outside* the grace window, or matching a family
  whose `revoked_at` is already set, revokes the whole family
  (`revoked_at = now()`) and returns a generic 401 — no distinct
  "theft detected" response body/code. The frontend treats any refresh
  failure the same today (and PACK-027 doesn't change that — see the
  frontend follow-up noted below), so there's no product surface yet that
  would consume a more specific signal.
- **Sliding expiry, not an absolute cap.** Every rotation resets
  `expires_at` to `now + 7 days`, matching the current single-JWT
  behavior's spirit (nothing forces re-login today short of 7 days of
  total inactivity). An absolute session cap was considered and rejected
  as unnecessary — no compliance driver for this personal app.
- **`Logout` now revokes server-side, not just cookie-clearing.** Not in
  the original audit finding's scope, but a natural small extension now
  that a table exists to revoke against — leaving `Logout` cookie-only
  next to a real revocation mechanism would read as an oversight, not a
  decision. `Logout` looks up the family from the presented cookie
  (best-effort — a missing/invalid cookie still just clears the cookie
  and returns 200, same as today) and sets `revoked_at`.
- **Cleanup: lazy, per-user sweep on login — no background job.** There's
  no scheduler/ticker infrastructure in this codebase at all yet, and no
  graceful shutdown (`PACK-021`, not started) to coordinate a background
  goroutine's lifecycle against — building a `time.Ticker` now would run
  forever with no clean stop, not actually the best-practice shape worth
  keeping as a template. Instead: `GoogleCallback` runs
  `DELETE FROM refresh_tokens WHERE user_id = $1 AND (revoked_at IS NOT
  NULL OR expires_at < now())` for that user before creating the new
  family row. No new process, bounded to one user's own dead rows. A
  proper periodic/global sweep is real future work once PACK-021's
  shutdown coordination exists to run it correctly — noted on PACK-021's
  own backlog entry in `master-spec.md` rather than left implicit.
- **Index included on `refresh_tokens.user_id`, ahead of PACK-025.**
  PACK-025 (DB indexes migration, not started) covers missing indexes on
  the *existing* tables project-wide as a generic sweep. This table's
  index isn't a candidate for that later sweep — it's load-bearing for
  this ticket's own hot path (every `/auth/refresh` call does a lookup
  scoped by `user_id`), so it ships in this ticket's migration, not
  deferred.
- **Manual verification via `requests/auth.http`, not a separate script.**
  `requests/README.md` had previously punted on this ("`/auth/refresh` and
  `/auth/logout` are cookie-based and could be automated, but that's its
  own piece of work, not done yet") on the assumption `.http` couldn't
  chain cookies. Confirmed otherwise: VS Code REST Client (the client in
  use) defaults `rest-client.rememberCookiesForSubsequentRequests` to
  `true`, so a real sequence works directly. `scripts/gen_token.go` is
  extended to also generate a refresh token, hash it, insert the matching
  `refresh_tokens` row, and write the raw token to `.env` as
  `DEV_REFRESH_TOKEN` — mirroring exactly what it already does for the dev
  access token — since there's no way to get a valid refresh cookie into a
  `.http` session otherwise (the real flow needs a Google OAuth browser
  round-trip).
- **Frontend gap found, explicitly not fixed in this ticket.** Mid-session
  refresh failure (`apiFetch`'s 401-retry path in
  `packing-list-react/src/lib/api/client.ts`) doesn't currently force a
  sign-out — the exception just propagates out of whatever query/mutation
  triggered it, leaving `AuthContext`'s `staleTime: Infinity` session
  query stale until a manual reload. This gap exists for any refresh
  failure today, but PACK-027 is what makes a *mid-session* revocation a
  real, reachable event for the first time. Fix is small (reuse
  `useLogout.ts`'s existing `setAccessToken(null)` +
  `queryClient.setQueryData(AUTH_SESSION_QUERY_KEY, null)` pattern, via
  the already-exported `queryClient` singleton in
  `src/lib/Tanstack/queryClient.ts`) — flagged as a `packing-list-react`
  follow-up to pick up once this ticket ships and there's a real revoked-
  family 401 to verify against, not bundled into this (Go-only) ticket.

## Acceptance criteria

**Repository layer (own commit, pause for review before the handler layer
— per this project's layer-by-layer implementation rule):**

- [x] Migration `000003_refresh_tokens` creates the `refresh_tokens` table
      (`id`, `user_id` FK `ON DELETE CASCADE`, `token_hash`,
      `previous_token_hash`, `previous_token_rotated_at`, `expires_at`,
      `revoked_at`, `created_at`) plus an index on `user_id`
- [x] `RefreshTokenRepository` interface (defined in `handler`, per this
      project's consumer-defined-interface convention) with:
      `CreateFamily(ctx, id, userID, tokenHash string, expiresAt time.Time)
      (*models.RefreshTokenFamily, error)` — `id` is caller-generated, not
      DB-default, since it must be known before minting the token that
      embeds it; `FindFamilyByID(ctx, id, userID string)
      (*models.RefreshTokenFamily, error)` — looks up by id (from the
      token's `familyId` claim), not by hash, returns `nil, nil` if no row
      matches (mirrors `GetCategoryByID`'s not-found convention);
      `RotateFamily(ctx, familyID, newTokenHash string, newExpiresAt
      time.Time) error` — shifts current hash into `previous_token_hash`
      (+ sets `previous_token_rotated_at`) and sets the new hash/expiry in
      one `UPDATE`; `RevokeFamily(ctx, familyID string) error` — sets
      `revoked_at`; `DeleteStaleFamiliesForUser(ctx, userID string) error`
      — deletes rows for that user where `revoked_at IS NOT NULL OR
      expires_at < now()`
- [x] `PostgresRefreshTokenRepository` in `internal/repository` implements
      the above
- [x] Integration test confirms `RotateFamily` correctly preserves the
      prior hash/timestamp in `previous_token_hash`/
      `previous_token_rotated_at` (not just overwriting them silently)
- [x] Integration test confirms `FindFamilyByID` returns the family for its
      own id/user, `nil, nil` for an unknown id, and `nil, nil` for a
      correct id under the wrong user
- [x] Integration test confirms `DeleteStaleFamiliesForUser` removes only
      revoked/expired rows for the given user, leaving active families
      (for that user and others) untouched

**Handler layer:**

- [x] `internal/auth/jwt.go`: `RefreshClaims{FamilyID string;
      jwt.RegisteredClaims}`; `GenerateRefreshToken(userID, familyID,
      secret)` / `ValidateRefreshToken` return `*RefreshClaims`
- [x] `GoogleCallback` generates a family id (`uuid.NewString()`), mints
      the refresh token with it embedded, calls
      `DeleteStaleFamiliesForUser` for the user, then `CreateFamily` with
      that same id and the token's hash
- [x] `RefreshToken` looks up the family by `claims.FamilyID` +
      `claims.Subject` via `FindFamilyByID`. Not found, or found with
      `revoked_at` already set → 401, generic error body (same shape as
      today's "invalid refresh token")
- [x] Hash matches `token_hash` → rotate: generate + hash a new refresh
      token (same family id), `RotateFamily`, set the new refresh cookie,
      issue a new access token, `200` (unchanged response shape:
      `{"accessToken": "..."}`)
- [x] Hash matches `previous_token_hash` within the 10s grace window →
      same rotation path as above (not flagged, not distinguished in the
      response)
- [x] Any other case (hash matches `previous_token_hash` outside the grace
      window, **or matches neither hash at all** — a token from 2+
      rotations ago) → `RevokeFamily`, 401. The "matches neither" branch is
      the fix for the gap manual verification found — see the Context
      section above
- [x] A revoked family's token immediately fails on any subsequent
      `/auth/refresh` call, not just the triggering one
- [x] `Logout` decodes the refresh cookie's `familyId` claim (if present
      and validly signed) and revokes that family directly — no lookup
      step needed, since revoking a nonexistent/already-revoked id is a
      harmless no-op. A missing or invalid cookie doesn't error — still
      clears the cookie and returns `200`, same as current behavior
- [x] `main.go` wiring: `NewAuthHandler` takes the new
      `RefreshTokenRepository`; route table unchanged (no new routes)

## Non-goals

- No ADR (see design-gate finding above — existing "Key architecture
  decisions" precedent used instead, updated at close-out)
- No absolute session-lifetime cap — sliding window only
- No distinct wire-contract signal for reuse-detected vs. any other
  invalid-refresh-token case — both return the same generic 401
- No global/background cleanup job — lazy per-user sweep on login only;
  a real ticker-based sweep is future work once PACK-021 lands (noted on
  PACK-021's own backlog entry)
- No `packing-list-react` changes — the mid-session force-sign-out gap
  found during this interview is a flagged follow-up ticket on that
  project's side, not part of this ticket
- No change to access token generation/validation or the 15-minute access
  token lifetime

## Expected test files

- `internal/repository/refresh_token_test.go` — (integration, against
  the real Neon dev DB per this project's convention):
  `TestCreateFamily_PersistsRow`, `TestFindFamilyByID_ReturnsFamily`,
  `TestFindFamilyByID_ReturnsNilForUnknownID`,
  `TestFindFamilyByID_ReturnsNilForWrongUser`,
  `TestRotateFamily_ShiftsCurrentIntoPrevious`,
  `TestRevokeFamily_SetsRevokedAt`,
  `TestDeleteStaleFamiliesForUser_RemovesOnlyRevokedOrExpiredForThatUser`
- `internal/handler/auth_handler_test.go` — extended existing file (mocked
  `RefreshTokenRepository` via `testify/mock`, per this project's
  override): `TestRefreshToken_RotatesOnCurrentHash`,
  `TestRefreshToken_RotatesWithinGraceWindowOnPreviousHash`,
  `TestRefreshToken_RevokesOnStaleReuseOutsideGraceWindow`,
  `TestRefreshToken_RejectsAlreadyRevokedFamily`,
  `TestRefreshToken_RevokesOnMultiGenerationStaleReuse` (added after
  manual verification caught the gap it covers),
  `TestGoogleCallback_CreatesFamilyAndSweepsStale`,
  `TestLogout_RevokesMatchingFamily`,
  `TestLogout_ClearsCookieEvenWithoutValidRefreshToken`
- `internal/auth/jwt_test.go` — extended `TestRefreshToken` to assert the
  new `FamilyID` claim round-trips
- `scripts/gen_token.go` — extended to also seed a `refresh_tokens` row
  and write `DEV_REFRESH_TOKEN` to `.env` (not itself a test file, but
  required setup for the manual verification below)
- `requests/auth.http` — new sequence: seed via
  `Cookie: refreshToken={{$dotenv DEV_REFRESH_TOKEN}}` on the first
  refresh, a second refresh relying on the cookie jar to prove rotation,
  a deliberate replay of the original `DEV_REFRESH_TOKEN` value to prove
  reuse detection (expect 401), a follow-up refresh proving the whole
  family is dead (expect 401), then `POST /auth/logout` followed by one
  more refresh proving server-side revocation (expect 401). Verified live
  against a running server via curl (equivalent to the `.http` file,
  mirroring PACK-034's precedent) — including the multi-generation-stale
  replay that first exposed the hash-only lookup gap.
