# PACK-017 — OAuth test isolation

## Context

Part of Epic 6 (Codebase Health & Hardening). Source findings: items 4
and 8 in `docs/handoffs/epic-6-findings.md`, bundled into one ticket
since both touch `internal/auth/google.go`/`google_test.go`:

4. Every test in `google_test.go` calls `NewGoogleOAuthManager`, which
   calls `oidc.NewProvider(ctx, "https://accounts.google.com")` — a real
   HTTPS call to Google's OIDC discovery endpoint on every test run.
8. `google_test.go` has a hand-rolled `contains(s, substr string) bool`
   helper duplicating `strings.Contains`.

Key decisions from the interview:

- **Checked what the existing tests actually exercise first**: all 4
  tests (`TestGenerateState`, `TestValidateState`,
  `TestValidateStateExpiry`, `TestGetAuthURL`) only touch `config` (via
  `GetAuthURL`) and the in-memory state store (`GenerateState`/
  `ValidateState`) — **none call `VerifyIDToken` or
  `ExchangeCodeForToken`**, the two methods that actually need a working
  `verifier`. That changed the shape of the fix.
- **Chose the simpler of two isolation approaches**: bypass OIDC provider
  construction in tests entirely, rather than standing up a fake local
  discovery server via `httptest` + a parameterized issuer URL pointed at
  the real `oidc.NewProvider`. The httptest-server approach would be more
  faithful to the production code path and would lay groundwork for
  eventually testing `VerifyIDToken` for real, but is meaningfully more
  test infrastructure (a discovery JSON document, a JWKS endpoint) for a
  ticket whose only current job is "stop hitting the network," not "add
  new coverage."
- **`GoogleOAuthManager`'s construction is split**: a new unexported
  `newGoogleOAuthManager(config *oauth2.Config, verifier
  *oidc.IDTokenVerifier) *GoogleOAuthManager` builds the struct from
  already-constructed pieces. The public `NewGoogleOAuthManager` is
  unchanged in behavior — still performs the real discovery call and
  builds `config`/`verifier` from it — but now delegates the final struct
  construction to the new helper. Tests get their own
  `newTestOAuthManager(clientID string) *GoogleOAuthManager` helper that
  builds a fake `*oauth2.Config` directly (no network, no OIDC library
  discovery call) and passes `nil` for the verifier.
- **`verifier` stays `nil` in test-constructed managers.** Since no test
  calls `VerifyIDToken`, this is safe today. If a future ticket adds
  `VerifyIDToken` coverage, it will need real verifier construction (a
  fake keyset or the httptest-server approach) — explicitly deferred, not
  this ticket's job.
- **No production code path changes.** `NewGoogleOAuthManager`'s
  signature, behavior, and `main.go`'s call site are all unchanged — this
  is purely a test-infrastructure refactor. No manual verification or
  `.http` file is warranted: there is no API-visible or runtime behavior
  change to check by hand.
- **`contains()` removal is unrelated but trivial** — replace its 3 call
  sites in `TestGetAuthURL` with `strings.Contains`, delete the local
  function, add the `"strings"` import.

## Acceptance criteria

- [x] **AC1 — `google_test.go` no longer makes a live network call.**
  - `internal/auth/google.go` gets the new unexported
    `newGoogleOAuthManager(config *oauth2.Config, verifier
    *oidc.IDTokenVerifier) *GoogleOAuthManager` constructor;
    `NewGoogleOAuthManager` delegates to it after real discovery,
    unchanged in behavior.
  - `google_test.go` gets `newTestOAuthManager(clientID string)
    *GoogleOAuthManager`, building a fake `*oauth2.Config` (arbitrary but
    plausible `AuthURL`/`TokenURL`, the same scopes as production) and
    calling `newGoogleOAuthManager(config, nil)`.
  - `TestGenerateState`, `TestValidateState`, `TestValidateStateExpiry`,
    `TestGetAuthURL` all construct their manager via
    `newTestOAuthManager` instead of `NewGoogleOAuthManager` — same
    assertions, no network I/O.
- [x] **AC2 — Hand-rolled `contains()` replaced with `strings.Contains`.**
  - The local `contains` function in `google_test.go` is removed.
  - Its 3 call sites in `TestGetAuthURL` use `strings.Contains` instead.

## Non-goals

- No test coverage added for `VerifyIDToken` or `ExchangeCodeForToken` —
  both remain untested; a `nil` verifier in `newTestOAuthManager` would
  panic if a future test calls `VerifyIDToken`. Deferred, not this
  ticket's job.
- No change to `NewGoogleOAuthManager`'s public signature or its
  production behavior; `main.go`'s call site is unaffected.
- No `UserId`→`UserID` casing, `parseName` duplication, `go.mod` `//
  indirect` cleanup, `validateTemplateItemNotes` rename — PACK-018.
- No `doRequest` retrofit — PACK-019.
- No `requests/*.http` structural rethink — PACK-020.
- No manual verification / no `.http` file — zero API-visible or runtime
  behavior change (see Context).

## Expected test files

- `internal/auth/google_test.go` (**modified**, no new test cases beyond
  what already exists — this ticket removes a network dependency from
  existing coverage, it doesn't add new behavioral coverage):
  - Add `"strings"`, `"github.com/coreos/go-oidc/v3/oidc"`, and
    `"golang.org/x/oauth2"` imports.
  - Add `newTestOAuthManager(clientID string) *GoogleOAuthManager` helper.
  - Update `TestGenerateState`, `TestValidateState`,
    `TestValidateStateExpiry`, `TestGetAuthURL` to construct their
    manager via the new helper instead of `NewGoogleOAuthManager`.
  - Remove `contains()`; update `TestGetAuthURL`'s 3 assertions to use
    `strings.Contains`.
- `internal/auth/google.go` (**modified**, not a test file): extract
  `newGoogleOAuthManager(config, verifier)`.
- No new `.http` file (see Non-goals).
