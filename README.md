# Pack-It — REST API

This is a REST API built in Go to serve a packing list app called Pack-It. I had really wanted to learn Go having spent years building full stack applications purely in TypeScript, so this simple app idea to solve a very specific and understandable problem was the perfect vessel to properly try my hand at building something real in a new language. AI assisted me in a heavily test-driven and interview/specs driven approach in order to allow me to be fully in control over the direction of the code. The result is a secure, mostly idiomatic Go application that I'm satisfied will provide a great template for me going forward in my Go journey!

The React frontend this project is designed to serve can be found [here](https://github.com/franciskershaw/packing-list-react), and goes into greater detail about the intended user experience. This API is deployed [here](https://api.packitapp.co.uk/health).

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Running Tests](#running-tests)
- [API Reference](#api-reference)
- [Project Structure](#project-structure)
- [Architecture Notes](#architecture-notes)

## Overview

What can this API actually do? It's a set of basic but powerful features allowing a user to sign in with Google, build
a personal library of items organised into categories (on top of a shared set of system-provided defaults), assemble reusable packing templates from that library, and ultimately generate packing lists for specific trips/events. These trips can optionally be seeded from a template, and they can tick items off on as they pack. This solves a minor issue I found I was having every time I packed for festivals (something I do every year), having to go through my own personal memory banks to identify what I'd need. Now I can look at last year's Green Man festival, or my festival template, and start from there instead of scratch.

Core use cases:

- Sign in with a Google account; no separate password/credential system. This was done for ease more than anything, adding an email address / password flow seemed like overkill for an app I only really anticipate myself and my wife using.
- Browse system-provided categories/items and extend them with personal ones.
- Build a named template (e.g. "Festivals" or "Beach trip") listing items with quantity/notes, organised by category.
- Create a packing list for an actual trip, optionally seeded from a template, then add/remove/adjust items independently of the template.
- Tick items off as packed while packing; bulk pack/unpack the whole list.
- Archive old lists instead of deleting them.

Out of scope for now: sharing/collaboration between users, password-based auth, and mobile/offline sync — this repo is the API only.

## Tech Stack

- **Language**: Go 1.26
- **Web framework**: [Gin](https://github.com/gin-gonic/gin)
- **Database**: PostgreSQL ([Neon](https://neon.tech)), accessed via
  [pgx](https://github.com/jackc/pgx)
- **Migrations**: [golang-migrate](https://github.com/golang-migrate/migrate),
  plain up/down SQL files in `db/migrations/`
- **Auth**: Google OAuth2/OIDC login (via
  [go-oidc](https://github.com/coreos/go-oidc) +
  `golang.org/x/oauth2`), custom JWT access/refresh tokens
  ([golang-jwt](https://github.com/golang-jwt/jwt))
- **Rate limiting**: [ulule/limiter](https://github.com/ulule/limiter)
- **Testing**: stdlib `testing` + [testify](https://github.com/stretchr/testify)
  (`testify/mock` for handler-layer repository mocks — see
  [Overrides of the global default process](CLAUDE.md) in `CLAUDE.md`)
- **Deployment**: Docker (multi-stage build to a distroless image), behind
  nginx

## Getting Started

Prerequisites: Go 1.26+, a PostgreSQL database (this project develops
against a [Neon](https://neon.tech) instance), and Google OAuth2
credentials.

```bash
# Clone and enter the repo
git clone <repo-url>
cd packing-list-go

# Copy the env template and fill in the values (see Environment Variables below)
cp .env.example .env

# Run migrations against your database
migrate -database "$DATABASE_URL" -path db/migrations up

# Start the server
go run main.go
```

The server starts on `PORT` (default `8080`) and exposes an unauthenticated
health check at `GET /health`.

For exercising the API by hand, see `requests/README.md` - a `.http` file
per resource, plus a `scripts/gen_token.go` helper that mints a dev bearer
token without going through the full Google OAuth round-trip.

## Environment Variables

All variables below are set via `.env` (see `.env.example`) and loaded
automatically at startup via `godotenv/autoload`.

| Variable                 | Required                   | Description                                                                                         |
| ------------------------ | -------------------------- | --------------------------------------------------------------------------------------------------- |
| `PORT`                   | No (default `8080`)        | Port the HTTP server listens on.                                                                    |
| `APP_ENV`                | No (default `development`) | `development` or `production` — controls Gin's mode.                                                |
| `DATABASE_URL`           | Yes                        | Postgres connection string.                                                                         |
| `JWT_SECRET_ACCESS`      | Yes                        | Signing secret for short-lived (15 min) access tokens.                                              |
| `JWT_SECRET_REFRESH`     | Yes                        | Signing secret for refresh tokens (7-day sliding expiry).                                           |
| `JWT_SECRET_OAUTH_STATE` | Yes                        | Signing secret for the OAuth `state` param, used to prevent CSRF during the Google login flow.      |
| `GOOGLE_CLIENT_ID`       | Yes                        | Google OAuth2 client ID.                                                                            |
| `GOOGLE_CLIENT_SECRET`   | Yes                        | Google OAuth2 client secret.                                                                        |
| `GOOGLE_REDIRECT_URI`    | Yes                        | Callback URL registered with Google (`/auth/google/callback`).                                      |
| `FRONTEND_URL`           | Yes                        | Origin allowed by CORS and used for post-login redirects.                                           |
| `TRUSTED_PROXIES`        | No                         | Comma-separated list of proxy IPs Gin should trust for client-IP resolution (e.g. behind nginx).    |
| `DEV_TOKEN`              | No                         | Auto-populated by `go run scripts/gen_token.go`; consumed by `requests/*.http`. Never set manually. |

## Running Tests

This project was created with a very strict emphasis on test-driven development. Where possible, each feature was created in this order:

- Repository (PostgreSQL) query tests, failing against non-implemented stubs
- Fixed repository queries, tested against a real Neon connection
- Handler logic tests, again failing at first against broken/stub handlers
- Fixed handlers, all tests passing
- Manual verification via .http files run using the VS Code REST Client extension

Handler tests are unit tests and need no database:

```bash
go test ./internal/handler/...
```

Repository tests are integration tests that hit a real database — never
spin up Docker or a local Postgres for these:

```bash
DATABASE_URL=$DATABASE_URL go test ./internal/repository/...
```

Lint with the same tool and flags CI runs, uncapped so it doesn't silently
truncate duplicate findings:

```bash
golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...
```

## API Reference

The full endpoint list lives in `main.go`'s route table and is documented
alongside acceptance criteria per ticket in
[`docs/specs/master-spec.md`](docs/specs/master-spec.md). At a glance:

| Resource      | Base path                                                                                                      |
| ------------- | -------------------------------------------------------------------------------------------------------------- |
| Auth          | `/auth/google/login`, `/auth/google/callback`, `/auth/refresh`, `/auth/logout`, `/me`                          |
| Categories    | `/categories`                                                                                                  |
| Items         | `/items`                                                                                                       |
| Templates     | `/templates` (+ nested `/templates/:id/items`)                                                                 |
| Packing lists | `/lists` (+ nested `/lists/:id/items`, `/lists/:id/pack-all`, `/lists/:id/unpack-all`, `/lists/:id/unarchive`) |

All routes except `/health` and the `/auth/*` group require a Bearer
access token. See `requests/*.http` for concrete, runnable examples of
every endpoint.

## Project Structure

```
config/           Environment/config loading
db/               DB connection setup, migrations, seed SQL
internal/
  auth/           Google OAuth manager, JWT issuing/verification
  handler/        HTTP handlers + the repository interfaces they consume
  middleware/     Auth, CORS, rate limiting, body-size limits, error logging
  models/         Domain types shared across handler/repository
  repository/     Postgres implementations of the handler-defined interfaces
  testutil/       Shared test helpers (e.g. auth header generation)
docs/
  specs/          Master spec + ticket backlog
  handoffs/       Per-ticket handoff docs written before implementation
requests/         Manual .http regression suite, one file per resource
scripts/          One-off dev tooling (e.g. gen_token.go)
```

## Architecture Notes

Key architectural decisions — the repository pattern (interfaces owned by
`handler`, not `repository`), the auth/token model, the ownership model for
system vs. user-owned rows, and soft-delete for packing lists — are recorded
in [`docs/specs/master-spec.md`](docs/specs/master-spec.md#key-architecture-decisions)
rather than duplicated here. See `LESSONS.md` for the retro log behind how
and why those decisions evolved.
