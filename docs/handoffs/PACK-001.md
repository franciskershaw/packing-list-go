# PACK-001 — Project scaffold, config, DB connection & migrations

> **Retroactive handoff.** Written 2026-07-07 during `project-kickoff`,
> after this code already existed. Reconstructed from `main.go`,
> `config/config.go`, `db/db.go`, `db/migrations/`, and the git log
> (`79ba053`, `d608319`, `f163bf4`). This is documentation of what was
> built, not a spec that preceded it — it did not go through the
> tests-first pipeline described in `~/.claude/CLAUDE.md`.

## Context

Bootstrap the Go service: load configuration from the environment, connect
to Postgres (Neon) and apply migrations, and stand up a Gin server with a
public health check so later tickets have somewhere to hang routes.

## Acceptance criteria

- [x] Config loads required values (DB connection string, JWT secrets,
      Google OAuth credentials, port) from environment variables, failing
      fast with a clear error if something required is missing.
- [x] `db.InitDB()` opens the Postgres connection and runs pending
      migrations from `db/migrations/`; `db.CloseDB()` releases it on
      shutdown.
- [x] `GET /health` returns 200 with a welcome message, unauthenticated.
- [x] Server exits non-zero with a logged error if config load, DB init, or
      `server.Run` fails, rather than panicking or failing silently.

## Non-goals / files not touched by this ticket

- No auth, no business-domain tables beyond the initial schema — this
  ticket only stands up the scaffold.

## Tests

None exist for this ticket's code (`config/config.go`, `db/db.go`,
`main.go`). Infra/bootstrap code of this kind is a reasonable candidate for
being exempted from the tests-first rule going forward — flag if a future
ticket touches this layer and needs the exemption revisited.
