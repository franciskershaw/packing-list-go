# PACK-032 — OAuth callback: redirect to frontend instead of returning JSON

## Context

`GoogleCallback` (`internal/handler/auth_handler.go`) currently sets the
refresh cookie and then renders the access token as JSON directly in the
browser tab. This was flagged by the 2026-07-11 API audit (finding S9:
"the frontend integration will need a deliberate redirect design — never
put the token in a URL") and formalized during the `packing-list-react`
project kickoff as
[that project's ADR 001](../../../packing-list-react/docs/adr/001-auth-session-model.md),
which documents the target flow end to end. This ticket implements the
backend half of that flow. It **blocks `packing-list-react`'s PACKFE-003**
(Google sign-in ticket) — that project's `/auth/callback` route doesn't
exist yet, which shapes this ticket's manual-verification scope (see
below).

**Design gate**: this is a new pattern for this repo — no existing route
redirects back to a configured external app after finishing an action
(`LoginWithGoogle`'s redirect goes *to* Google, not back from it). Raised
explicitly during the interview; the call was **not** to write a
dedicated ADR for it — the blast radius was judged small enough (one
config field, one handler's success path) for this handoff doc's context
section to carry the documentation instead. Recorded here so that
decision is explicit, not an oversight.

Key decisions from the interview (2026-07-17):

- **Redirect target**: new `FRONTEND_URL` config value, **required** —
  `config.Load()` fails startup if unset, same tier as `DATABASE_URL`/JWT
  secrets. This matches the repo's established fail-loud convention
  (the `DATABASE_URL` silent-skip incident, see `LESSONS.md`) over a
  silently-defaulted dev convenience value.
- **Only the success path changes.** Existing failure paths (invalid
  state, missing code/state, token exchange failure, ID-token verify
  failure, user-processing failure) keep returning JSON error bodies
  exactly as today. Designing a frontend-facing error-redirect contract
  now would be speculative — `packing-list-react` has no `/auth/callback`
  route or error UI yet to redirect into (that's `PACKFE-003`, which this
  ticket blocks). Flag as a candidate follow-up once `PACKFE-003` is
  underway and has somewhere real to send an error to.
- **Redirect status**: `http.StatusTemporaryRedirect` (307), matching
  `LoginWithGoogle`'s existing redirect one function up in the same file
  — direct precedent, no new status-code convention introduced.
- **Cookie `SameSite=Lax`: no change.** The existing code comment on
  `setRefreshCookie` says "revisit if a cross-origin frontend consumer is
  added" — this ticket *is* that consumer, so it was evaluated directly
  rather than left stale. `packing-list-react`'s local-dev design (Vite
  proxy: the browser only ever talks to its own `:5173` origin, which
  forwards server-side to `:8080`) means the refresh cookie is only ever
  sent browser-to-`:8080` directly during this one top-level GET
  redirect, which `SameSite=Lax` always permits regardless of site.
  The comment gets updated to reflect that this is the consumer it was
  watching for and that Lax remains correct under the current dev
  design; genuine cross-origin cookie behavior stays deferred to
  whenever a production deployment target is chosen (already an open
  "revisit when" in `packing-list-react`'s ADR 001 — not re-decided
  here).
- **Manual verification is redirect-plus-cookie only, not a full round
  trip.** Since `packing-list-react`'s `/auth/callback` doesn't exist
  yet, the browser will hit a connection error after landing on
  `:5173/auth/callback` — that's expected and out of scope for this
  ticket. Verification confirms the redirect and cookie are correct via
  browser dev tools, not that a frontend page renders.
- **Known simplification, not fixed here**: `FRONTEND_URL` is
  concatenated directly (`cfg.FrontendURL + "/auth/callback"`) with no
  trailing-slash normalization. The operator is expected to set it
  without a trailing slash, per `.env.example`'s documented format
  (`http://localhost:5173`, no trailing `/`). Not worth defensive code
  for an operator-controlled config value.

## Acceptance criteria

- [ ] `config.Config` gains a `FrontendURL string` field.
- [ ] `config.Load()` reads `FRONTEND_URL` from the environment and
      returns an error if it's unset — same tier as `DATABASE_URL`,
      `JWT_SECRET_ACCESS`, `JWT_SECRET_REFRESH`.
- [ ] `.env.example` gets a `FRONTEND_URL=http://localhost:5173` line.
- [ ] `GoogleCallback`'s success path: after setting the refresh cookie
      (unchanged), responds with `http.StatusTemporaryRedirect` (307) to
      `{FRONTEND_URL}/auth/callback` instead of `c.JSON` with
      `accessToken`/`user` in the body. No token or user data appears
      anywhere in the response body or the `Location` URL.
- [ ] `GoogleCallback`'s failure paths (invalid state, missing
      code/state, exchange failure, verify failure, user-processing
      failure) are **unchanged** — still return their existing JSON error
      bodies. Explicit non-regression, not new behavior.
- [ ] `setRefreshCookie`'s comment is updated to reflect that this
      ticket is the "cross-origin frontend consumer" it was watching for,
      and that `SameSite=Lax` remains correct under the current
      Vite-proxy dev design (cite `packing-list-react`'s ADR 001 for the
      full reasoning). `SameSite`/`Secure` behavior itself is unchanged.
- [ ] Manual verification (see below) confirms the redirect and cookie
      are correct against a real Google OAuth round trip.

## Non-goals

- `packing-list-react`'s own `/auth/callback` route, `AuthContext`, or
  any frontend code — that's `PACKFE-003`, not this ticket.
- Changing failure-path responses to redirects — deferred per the
  interview decision above.
- Any `SameSite`/`Secure` cookie policy change — deferred to a future
  production-deployment ticket.
- A production value for `FRONTEND_URL` — undecided (deployment target
  is still "undecided, local only" per `packing-list-react`'s master
  spec NFR section). Only the dev value is set now.
- `PACK-026` (OpenAPI spec) and `PACK-027` (refresh-token rotation) —
  unrelated, not touched here.
- Trailing-slash normalization of `FRONTEND_URL` — see "known
  simplification" above.
- A dedicated ADR — considered and explicitly declined during the
  interview; see "Design gate" above.

## Expected test files

- **`config/config_test.go`**
  - `setRequiredEnv(t)` helper updated to also set `FRONTEND_URL`, so the
    two existing tests (`TestLoad_EnvironmentDefaultsToDevelopment`,
    `TestLoad_EnvironmentReadsAppEnv`) keep passing once the field
    becomes required.
  - New `TestLoad_RequiresFrontendURL` — unsets `FRONTEND_URL` (with the
    other required vars set), asserts `Load()` returns an error. Traces
    to AC2 (required-field validation).
- **`internal/handler/auth_handler_test.go`**
  - `testConfig(environment)` helper updated to set
    `FrontendURL: "http://localhost:5173"`, so it compiles against the
    new `Config` field and existing tests keep working.
  - `TestGoogleCallback_HappyPath` — assertions change from "200 + JSON
    body with `accessToken`/`user`" to "307 status, `Location` header
    equals `http://localhost:5173/auth/callback`, refresh cookie still
    set with the same attributes as before, body contains no
    `accessToken`/`user` data." Traces to AC4.
  - `TestGoogleCallback_HappyPath_SecureCookieInProduction` — same
    redirect/`Location` assertions added; existing `Secure=true`
    cookie assertion under a production config is preserved unchanged.
    Traces to AC4 (redirect) and is a regression guard for the existing
    Secure-cookie behavior (not new coverage).
  - `TestGoogleCallback_InvalidState`, `TestGoogleCallback_MissingCode`,
    `TestGoogleCallback_MissingState` — **left as-is, no changes
    expected.** These exist to guard the documented design decision that
    failure paths are intentionally unchanged (AC5), not to cover
    otherwise-untested new behavior — flagging that explicitly per the
    global process rule on test-to-AC traceability.
- **Manual verification** (no new `.http` file — `requests/auth.http`'s
  README already documents that login/callback need a real browser
  round-trip and aren't automatable this way):
  1. Set `FRONTEND_URL=http://localhost:5173` in your local `.env`.
  2. Start the server (`go run main.go`), and click through
     `GET /auth/google/login` in a real browser, completing the actual
     Google consent screen.
  3. In browser dev tools' network tab, confirm on the callback request:
     status `307`, `Location` header exactly
     `http://localhost:5173/auth/callback`, a `refreshToken` cookie
     present with `HttpOnly`, `SameSite=Lax`, `Secure=false` (dev).
  4. Confirm no `accessToken` or user data appears anywhere in the
     response headers or body.
  5. The browser landing on a connection error at `:5173/auth/callback`
     afterward is **expected** — nothing listens there until `PACKFE-003`
     ships. Not a failure of this ticket.
