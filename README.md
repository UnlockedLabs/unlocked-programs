# unlocked-programs

Programs: programs, their classes and cohorts, enrollments, class events and
attendance, rooms and instructors, the Programs reports, and the Canvas and
Essential Education classes shown alongside them. It was a module inside
unlocked-core until EN-207, and its routes answer exactly as they did there.

Its pages are still in core's frontend. The ingress sends their API paths
here (`/api/programs`, `/api/program-classes`, `/api/classes`,
`/api/program-users`, `/api/program-facilities`, `/api/instructors`,
`/api/rooms`, `/api/admin-calendar`, `/api/student-calendar`,
`/api/reports`) and everything else to core. Five routes shared a path with
core's own routes, so they moved to new paths, which core's frontend calls:

| Was | Now |
|---|---|
| `GET /api/users/{id}/programs` | `GET /api/program-users/{id}/programs` |
| `GET /api/users/{id}/attendance-trend` | `GET /api/program-users/{id}/attendance-trend` |
| `GET /api/users/{id}/attendance-export` | `GET /api/program-users/{id}/attendance-export` |
| `GET /api/users/{id}/program-completions` | `GET /api/program-users/{id}/program-completions` |
| `GET /api/facilities/{facilityId}/instructors` | `GET /api/program-facilities/{facilityId}/instructors` |

## Commands

One binary, one image:

| Command | What it does |
|---|---|
| `app serve` | The API, on `ADDR`, and the answers to core's questions. Waits until the database is migrated. |
| `app worker` | Class activation: every morning at 5, and once at start outside dev. Run one replica, so it runs once. Waits until the database is migrated. |
| `app migrate` | Runs the migrations. Run it after core's migrations. |
| `app seed-dev` | Local only (`APP_ENV=dev`): adds sample programs, classes, enrollments and attendance for core's residents. Core's `make seed` runs it. |
| `app migrate-reset` | Local only (`APP_ENV=dev`): rolls back the migrations. Core's `make migrate-fresh` runs it before its own reset. |

## Code

Laid out like Open Content:

| Package | What it holds |
|---|---|
| `cmd/app` | The commands above. |
| `internal/models` | The tables as Go types, the rows of core's tables Programs uses, and the shapes the routes read and answer with. |
| `internal/database` | The connection, migrations, sample data and every query, on `database.DB`. |
| `internal/handlers` | The routes and their access checks, the Canvas and Essential Education classes, the reports and their PDFs, metrics, and the answers to core's questions (`internal_api.go`). |
| `internal/tasks` | The worker: class activation and its schedule. |
| `internal/server` | The HTTP server: health check, version, graceful stop. |
| `internal/integration` | The tests that came from core's `tests/integration`. |
| `templates` | The report templates JasperStarter renders. |

## Run it locally

Core's `make dev` builds and runs Programs next to core, from `main` on
GitHub, or from this checkout when core's `.env` sets `PROGRAMS_DIR` to it,
so most of the time that's all you need. To run it on its own you need Go
1.26 and Docker, plus core's Postgres, NATS and Kratos, and the Provider
Platforms service.

```sh
make migrate  # migrations
make run      # API on :8080
make worker   # the worker
make test     # go test -race
make lint     # golangci-lint, in Docker
make build    # the image, named after the repo
```

Settings, all from the environment:

| Variable | What it does |
|---|---|
| `ADDR` | Where the API listens. Default `:8080`. |
| `APP_DSN`, or `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Core's database, as core reads them. |
| `NATS_URL`, `NATS_USER`, `NATS_PASSWORD` | Core's NATS, which caches the Canvas classes. |
| `KRATOS_PUBLIC_URL` | Kratos's public API, to check the login session. |
| `APP_URL` | Core inside the cluster, such as `http://server:8080`: who a user is, and which features are on at a facility. |
| `PROGRAMS_TOKEN` | The site's secret core asks its questions with. Core's server has the same one. `serve` won't start without it. |
| `PROVIDER_PLATFORMS_URL` | The Provider Platforms service. Default `http://provider-platforms:8080`. |
| `PROVIDER_PLATFORMS_TOKEN` | The secret Programs asks Provider Platforms with, the same one core uses. `serve` won't start without it. |
| `JASPER_TEMPLATE_DIR` | The report templates. Default `/templates`, where the image has them. |
| `JASPER_TEMP_DIR` | Where a report's working files go. Default a folder in the system's temp directory. |
| `APP_ENV` | `dev` for local runs: no HSTS, no class activation at start, and `seed-dev` and `migrate-reset` allowed. |
| `LOG_LEVEL` | Default `info`. Logs are JSON, as core's are. |
| `APP_VERSION`, `GIT_COMMIT`, `GIT_BRANCH` | What `/api/version` reports. CI bakes them into the image. |

## How it works with core

- **Login:** each request's Kratos session is checked with
  `identity.SessionChecker` from unlocked-contracts, which asks core for the
  user's claims, feature flags included. The routes then run core's checks,
  with core's answers: the route's own check (the user in the path, or a
  class, program or facility in the caller's facility), admin, then the
  route's features (`internal/handlers/middleware.go`).
- **Core's questions:** core's users list, resident transfer and
  deactivation, usage report, dashboard, facility list and delete check and
  account history ask Programs through `unlocked-contracts/programs`. This
  service answers them on `/internal/v1/programs/`, which the ingress never
  routes, and only with `PROGRAMS_TOKEN`. Ending enrollments for a transfer
  and withdrawing them on deactivation change nothing the second time, so
  core can retry them.
- **Provider Platforms:** Programs asks the Provider Platforms service for
  providers and the users linked to them, with
  `unlocked-contracts/providerplatforms`. Provider Platforms asks core for an
  Essential Education provider's users and to clear the cached Canvas
  programs when a link changes, and core passes those on to Programs
  (`LiveUsers`, `MappingsChanged`).
- **Canvas and Essential Education** are read live from the providers' APIs.
  The Canvas programs, classes and events are cached in NATS key-value
  buckets for five minutes. In the list of every facility's classes, a
  facility's Canvas classes are left out when it has Provider Platforms
  turned off, which `features.Checker` asks core.
- **Report PDFs** are rendered by JasperStarter, which the image has: it's
  built on core's `golang-jasper` image. Core keeps its own usage report.
- **Metrics:** `/api/metrics` serves core's request metrics for Prometheus,
  labeled by route, and the ingress never routes it.

## Database

Programs' tables live in core's database, in the `programs` schema. Its
queries run as the `programs` role: the connection switches with
`SET ROLE programs` and `search_path = programs, public`, so the service logs
in with core's database user and needs no password of its own. The role can:

- read and write only the `programs` schema, where its change log,
  `change_log_entries`, can be read, added to and deleted from, but never
  changed;
- read core's `users` and `facilities`;
- read and add to (never change) core's `user_account_history`, so recording
  attendance writes its history row in the same transaction.

Postgres refuses anything else, which is why core asks Programs for its
numbers instead of reading its tables.

- **Migrations** are in `internal/database/migrations` and run as core's
  database user. Their ledger is `public.goose_db_version_programs`, the one
  core used, so a site that ran them inside core doesn't run them again.
  Never edit a merged migration; add a new one.
- **Create new tables as `programs.<name>`.** Migrations run as core's user,
  so an unqualified table lands in `public`, where the role can't use it.
- **The enum types** (`class_status`, `credit_type`, `funding_type`,
  `program_type`) stay in `public`.

## Tests

- `internal/handlers/routes_test.go` pins every route's status, JSON, access
  check and feature flags, and the Canvas and Essential Education paths
  through a fake LMS, against the saved responses in
  `internal/handlers/testdata/routes`, which came unchanged from core.
  After an intended change, regenerate them and review the diff:

  ```sh
  go test ./internal/handlers -run 'TestRoute' -update
  ```

- `internal/integration` has core's other Programs tests: classes, events,
  attendance, conflicts, delete guards, reports and the dashboard numbers.
- A fake Provider Platforms (`internal/testutil`) answers from tables the
  tests fill, through the real contracts client and handler.
- `TestJasperReports` renders every report with JasperStarter. It needs
  jasperstarter, so CI skips it. Run it in core's image:

  ```sh
  docker run --rm -v "$PWD":/src -w /src -e GOTOOLCHAIN=auto \
    ghcr.io/unlockedlabs/golang-jasper:latest \
    go test ./internal/handlers -run TestJasperReports
  ```

- The tests run on SQLite without NATS or the providers, so they can't run
  the Postgres-only queries (pinned as 500s in the route tests), the NATS
  caches, the schema and role, or the migrations. Check those against core's
  `make dev`.

## How to release

1. Merge PRs to `main` with conventional-commit titles (`feat: ...`,
   `fix: ...`). Each merge pushes `:sha-<short>` to ECR, then moves `:latest`
   to it if it's still the tip of `main`.
2. release-please keeps a `chore(main): release X.Y.Z` PR open with the
   changelog. A `feat` bumps the minor version, a `fix` the patch (before
   1.0.0, a breaking change bumps the minor version).
3. Merge that PR when you want a release. It tags `vX.Y.Z`, and CI pushes
   `:X.Y.Z` to ECR.

The `conventional commits` and `pr title` checks allow `feat`, `fix`, `perf`,
`revert`, `chore`, `docs`, `style`, `refactor`, `test`, `build` and `ci`. A
`!` after the type marks a breaking change.

## How to deploy

The templates live in `unlocked-platform` (added in its EN-207 PR): the
`serve` Deployment, with its readiness probe on `/api/healthcheck`, and
Service; the `worker` Deployment with one replica; a migrate Job that runs
after core's; and the ingress paths above. Pin the release in each site file
and deploy through the deploy workflow. Only staging may use `latest`.

## Vulnerability alerts

`govulncheck` runs on every push and blocks a PR with a finding. When it
fails on `main`, it posts to Slack through the `SLACK_WEBHOOK_URL`
organization secret; without the secret, the failure shows only as a red
check.
