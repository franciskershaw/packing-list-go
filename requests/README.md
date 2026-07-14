# Manual `.http` regression suite

One file per resource, run top-to-bottom in VS Code with the [REST
Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client)
extension. This is the manual regression pass for the whole API — run it
before closing out a phase of work, not on every commit.

## Setup (once per session)

1. Start the server: `go run main.go`
2. Generate a token: `go run scripts/gen_token.go`

That's it — the script upserts a `DEV_TOKEN` line into `.env`, and every
request in every file picks it up automatically via
`{{$dotenv DEV_TOKEN}}`. There's nothing to paste. Re-run the script
whenever the token expires (15 minutes) or the server restarts with a new
`JWT_SECRET_ACCESS`.

The host also resolves from `.env`, reusing the same `PORT` the server
binds to (`config.Config`) — every request URL is
`http://localhost:{{$dotenv PORT}}/...`. If you change `PORT` in `.env`,
both the server and every `.http` file pick it up with no further edits.

## Running a file

Each file is meant to be run **top-to-bottom, once, per session**.
Sections chain off `@name`-captured variables from earlier in the same
file (e.g. a category created near the top is reused by every later
section that needs one) — running a request out of order, or re-running
the file without restarting from the top, will generally fail because the
captured IDs point at rows that were renamed/deleted/archived by requests
further down.

Each file ends with a **Cleanup** section (or, where the resource's own
`DELETE` happy-path test already removes what was created — see
`categories.http` — no separate section is needed) that removes everything
the file created, so the database is left in the same state it started in.
Re-running a file from a clean server should behave the same way twice in
a row.

## What's deliberately not covered

- **401 checks** appear once or twice per file (a representative read and
  a representative write), not after every single endpoint — the auth
  middleware is shared/global, so one or two checks per file already prove
  it's wired up without repeating the same assertion at every route.
- **`requests/auth.http` only covers `GET /me`.** The Google OAuth
  login/callback flow needs a real browser round-trip and can't be driven
  by a plain `.http` request. `/auth/refresh` and `/auth/logout` are
  cookie-based and could be automated, but that's its own piece of work,
  not done yet.
- There's no separate "smoke test" file. If a fast per-commit sanity
  check becomes a real need again (vs. this being a full pre-close-out
  regression pass), that's worth its own design rather than retrofitting
  it into these files.
