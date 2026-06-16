# ainyx-user-api

A production-quality REST API for managing users, built with **GoFiber**, **MySQL**, and **SQLC**. Each user has a name and date of birth; the API computes the user's age on read.

## Features

- **Full CRUD** — create, read, update, and delete users over a clean REST API.
- **Computed age on read** — age is derived from `dob` at request time (never stored), so it can never go stale. Pure, unit-tested `CalculateAge` with correct leap-year handling.
- **Pagination** — `GET /users?page=&limit=` returns a data envelope plus `{page, limit, total}` metadata; `limit` is clamped to 1–100.
- **Strict DOB validation** — format, future-date, pre-1900, and minimum-age checks, each with its own clear `400` message.
- **Structured logging (Uber Zap)** — JSON in production, human-readable in development; error-level logs carry stack traces.
- **Request correlation** — every request gets an `X-Request-ID` (honored if the client sends one), echoed in responses, logs, and error bodies.
- **Centralized error envelope** — all failures return `{ "error": ..., "request_id": ... }` with the correct HTTP status.
- **Type-safe data layer (SQLC)** — hand-written SQL compiled into type-checked Go; no ORM, no runtime reflection.
- **Connection pooling** — tuned `*sql.DB` pool (max/idle conns, conn lifetime) with a startup ping-retry loop for slow-starting databases.
- **Graceful shutdown** — listens for `SIGINT`/`SIGTERM` and drains in-flight requests with a 10s timeout.
- **Health endpoint** — `GET /health` pings the DB and reports `200`/`503` for orchestrators and load balancers.
- **Dockerized** — multi-stage build (tiny non-root Alpine runtime) and a one-command `docker compose up` that brings up MySQL + the app with a health-gated startup order.
- **Zero-dependency frontend** — a single-file `frontend/index.html` console that dynamically lists, creates, edits, and deletes users without a page refresh.
- **CORS enabled** — so the static frontend can call the API directly.

## Tech Stack

| Concern        | Choice                          |
|----------------|---------------------------------|
| HTTP framework | GoFiber v2                      |
| Database       | MySQL 8.0                       |
| Query codegen  | SQLC v1.31.1                    |
| Logging        | Uber Zap (structured)           |
| Validation     | go-playground/validator v10     |
| Migrations     | golang-migrate format           |
| Container      | Docker + docker-compose         |

## Quick Demo

The server runs locally without Docker — see "Running Locally" section.
Docker support is included for containerized deployment.

## Screenshots

> Screenshots live in the [`screenshots/`](screenshots) folder. See
> [`screenshots/README.md`](screenshots/README.md) for exactly what each one should capture.

### Dashboard — user list
The single-file frontend console showing the paginated users table.

![Dashboard](screenshots/dashboard.png)

### Create a user
Creating a user from the form; the table refreshes automatically (no page reload).

![Create user](screenshots/create-user.png)

### Update a user
Editing an existing user in-place ("Edit User #N" mode).

![Update user](screenshots/update-user.png)

### Delete a user
Deleting a user with a confirmation prompt; the table updates live.

![Delete user](screenshots/delete-user.png)

### API response (curl)
A raw API call and its JSON response, including the computed `age` field.

![API response](screenshots/api-response.png)

### Running in Docker
Both containers (`ainyx-mysql`, `ainyx-app`) up and healthy via `docker compose`.

![Docker running](screenshots/docker-running.png)

### Tests passing
`go test ./... -v` with all age + DOB validation subtests passing.

![Tests passing](screenshots/tests-passing.png)

## Architecture

Clean layered design — dependencies flow inward, each layer is independently testable:

```
cmd/server/main.go      → wiring, DB connect, graceful shutdown
internal/routes         → route registration
internal/middleware     → request id, zap logger, CORS, error handler
internal/handler        → HTTP parsing + validation
internal/service        → business logic + age calculation (pure, tested)
db/sqlc                 → generated, type-safe queries
internal/models         → request/response DTOs
config, internal/logger → configuration + logger setup
```

## Request Flow

Every request travels through the same layered pipeline. Each layer has one job
and hands off to the next, which keeps the code testable and easy to reason about.

```
        ┌─────────────┐
HTTP →  │  Middleware │   RequestID → CORS → Zap logger   (cmd/server/main.go)
        └──────┬──────┘   attaches X-Request-ID, logs the request
               ▼
        ┌─────────────┐
        │   Routes    │   maps method + path → handler     (internal/routes)
        └──────┬──────┘
               ▼
        ┌─────────────┐
        │   Handler   │   parse body, validate shape,      (internal/handler)
        └──────┬──────┘   map errors → HTTP status
               ▼
        ┌─────────────┐
        │   Service   │   business rules: DOB validation,  (internal/service)
        └──────┬──────┘   age calculation, sentinel errors
               ▼
        ┌─────────────┐
        │  db/sqlc    │   type-safe generated queries      (db/sqlc)
        └──────┬──────┘
               ▼
        ┌─────────────┐
        │    MySQL    │   connection pool (*sql.DB)
        └─────────────┘
```

**Worked example — `POST /users`:**

1. **Middleware** assigns an `X-Request-ID`, then logs the incoming request.
2. **Router** matches `POST /users` → `UserHandler.Create`.
3. **Handler** (`user_handler.go`) parses the JSON body and runs struct validation
   (`name` 2–100 chars, `dob` is `YYYY-MM-DD`). Bad input → `400` immediately.
4. **Service** (`user_service.go`) applies the deeper DOB rules (not future,
   not before 1900, at least 1 year old) and persists via SQLC.
5. **db/sqlc** runs the generated `CreateUser` query against MySQL; the DB assigns
   the auto-increment `id`, read back with `LastInsertId()`.
6. The response bubbles back up; on any error, the **centralized error handler**
   wraps it in `{ "error": ..., "request_id": ... }` with the right status code,
   and the logger records the outcome.

**On a read (`GET /users/:id` or `GET /users`)**, step 4 additionally computes
`age` from `dob` via the pure `CalculateAge` function before returning — the age
is never read from the database because it isn't stored there.

## API Endpoints

| Method | Path          | Description                          | Success |
|--------|---------------|--------------------------------------|---------|
| POST   | `/users`      | Create a user                        | 201     |
| GET    | `/users/:id`  | Get a user (includes `age`)          | 200     |
| GET    | `/users`      | List users, paginated (`?page=&limit=`) | 200  |
| PUT    | `/users/:id`  | Update a user                        | 200     |
| DELETE | `/users/:id`  | Delete a user                        | 204     |
| GET    | `/health`     | Health check (DB ping + timestamp)   | 200/503 |

### Examples

**Create**
```bash
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","dob":"1990-05-10"}'
# 201 -> {"id":1,"name":"Alice","dob":"1990-05-10"}
```

**Get (with age)**
```bash
curl http://localhost:3000/users/1
# 200 -> {"id":1,"name":"Alice","dob":"1990-05-10","age":35}
```

**List (paginated)**
```bash
curl "http://localhost:3000/users?page=1&limit=10"
# 200 -> {"data":[{...,"age":35}],"pagination":{"page":1,"limit":10,"total":1}}
```

**Update**
```bash
curl -X PUT http://localhost:3000/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Updated","dob":"1991-03-15"}'
# 200 -> {"id":1,"name":"Alice Updated","dob":"1991-03-15"}
```

**Delete**
```bash
curl -X DELETE http://localhost:3000/users/1
# 204 No Content
```

**Health**
```bash
curl http://localhost:3000/health
# 200 -> {"status":"ok","db":"connected","timestamp":"2026-06-13T14:20:49Z"}
```
The endpoint pings the database. If the DB is unreachable at runtime it returns
`503` with `{"status":"degraded","db":"disconnected","timestamp":"..."}`.

### Error format

All errors share one envelope, with the request id for correlation:

```json
{ "error": "user not found", "request_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479" }
```

Notable error responses:

| Scenario                              | Status | Message               |
|---------------------------------------|--------|-----------------------|
| Non-numeric id in URL (any endpoint)  | 400    | `invalid user id`     |
| `GET`/`PUT`/`DELETE` unknown user     | 404    | `user not found`      |

## Validation Rules

- `name`: required, 2–100 characters.
- `dob`: required. The following rules are enforced, each with a specific
  `400 Bad Request` message:

  | Condition                       | Error message                              |
  |---------------------------------|--------------------------------------------|
  | Not `YYYY-MM-DD` format         | `use format YYYY-MM-DD`                     |
  | In the future                   | `date of birth cannot be in the future`    |
  | Before `1900-01-01`             | `date of birth seems invalid`              |
  | Less than 1 year old            | `person must be at least 1 year old`       |

## Age Calculation

Computed in `internal/service/age.go` as a pure function `CalculateAge(dob, now)`:
the raw year difference, minus one if the birthday hasn't occurred yet this year.
Leap-year birthdays (Feb 29) are handled by calendar comparison — in non-leap years
the birthday is treated as occurring on Mar 1.

## Getting Started — Full Setup (start to end)

There are two ways to run the project. **Option A (Docker)** is the fastest — it
starts MySQL and the app together with one command and needs no local Go or MySQL.
**Option B (local)** runs the Go binary directly against your own MySQL.

### Prerequisites

| For Docker (Option A) | For local (Option B)              |
|-----------------------|-----------------------------------|
| Docker Desktop        | Go 1.26+                          |
| (running)             | MySQL 8.0 (running)               |
|                       | `sqlc` (only if regenerating queries) |

### Step 0 — Get the code

```bash
git clone https://github.com/Vamsi-Kathi/ainyx-user-api.git
cd ainyx-user-api
```

---

### Option A — Run with Docker (recommended)

This brings up MySQL (with a health check) and the app together. The schema is
auto-applied on first boot via an init script mounted into the MySQL container —
no manual migration needed.

1. **Start Docker Desktop** and make sure it is running.
2. **Create your `.env`** — Compose reads `MYSQL_ROOT_PASSWORD` from it and will
   refuse to start if it is missing (there is no hardcoded default):
   ```bash
   cp .env.example .env
   # then edit .env and set MYSQL_ROOT_PASSWORD to a password of your choice
   ```
3. **Build and start everything:**
   ```bash
   docker compose up --build
   ```
4. **Wait** until you see the app log `server starting` and `connected to database`.
5. **Verify it's up:**
   ```bash
   curl http://localhost:3000/health
   # -> {"status":"ok","db":"connected","timestamp":"..."}
   ```

| Service | URL / Address                                                                 |
|---------|-------------------------------------------------------------------------------|
| App     | http://localhost:3000                                                         |
| MySQL   | localhost:**3307** (container's 3306 published on host 3307 to avoid clashes) |

**Stop / reset:**
```bash
docker compose down          # stop containers
docker compose down -v       # stop AND wipe the database volume (fresh schema next boot)
```

> **Note on Go version:** the assignment suggested `golang:1.22-alpine` for the
> final image, but this module targets Go 1.26, which cannot be compiled by 1.22.
> The Dockerfile therefore uses `golang:1.26-alpine` as the builder and a minimal
> `alpine:3.20` runtime image (smaller and more secure than shipping the full Go image).

---

### Option B — Run locally (without Docker)

Prerequisites: Go 1.26+, a running MySQL 8.0 with a database named `ainyx_users`.

1. **Copy the env file** and adjust values to match your local MySQL:
   ```bash
   cp .env.example .env
   ```
2. **Create the database** (if it doesn't exist) and **apply the schema** — either:
   - Run the SQL in `db/migrations/000001_create_users_table.up.sql` against your DB, **or**
   - Use golang-migrate:
     ```bash
     migrate -path db/migrations \
       -database "mysql://root:yourpassword@tcp(localhost:3306)/ainyx_users" up
     ```
3. **Download dependencies:**
   ```bash
   go mod download
   ```
4. **Start the server:**
   ```bash
   go run ./cmd/server
   ```
   Server listens on `http://localhost:3000`.
5. **Verify:**
   ```bash
   curl http://localhost:3000/health
   ```

---

### Step — Use the frontend

A zero-dependency, single-file console lives at `frontend/index.html`. With the
API running, open that file in a browser (it defaults to `http://localhost:3000`)
to create, list, edit, and delete users. The table updates live — no page refresh.

---

### Step — Try the API

See the [Examples](#examples) section above for ready-to-paste `curl` commands
covering create, get, list, update, delete, and health.

## Running the Tests

The project ships with unit tests for the age calculation and DOB validation
(both pure, deterministic, no database required).

```bash
# Run the whole suite:
go test ./...

# Verbose — see every subtest name and PASS/FAIL:
go test ./... -v

# Just the service package (age + DOB), verbose:
go test ./internal/service -v
```

Expected: **all tests pass** (20 subtests across 2 table-driven tests). Coverage:

| Test file                          | Subtests | What it checks                                          |
|------------------------------------|:--------:|---------------------------------------------------------|
| `internal/service/age_test.go`     |    9     | born today, exact N years, pre/post birthday, leap-year Feb 29, future-date clamping |
| `internal/service/dob_test.go`     |    11    | format, future-date, pre-1900, min-age, and valid-DOB acceptance |

To also check the code compiles cleanly and is well-formed:

```bash
go vet ./...
gofmt -l .   # prints nothing if all files are formatted
```

## Code Generation (SQLC)

Queries live in `db/sqlc/query.sql`; the schema in `db/migrations`. After editing
either, regenerate the type-safe Go with:

```bash
sqlc generate
```

## Project Layout

```
ainyx-user-api/
├── cmd/server/main.go
├── config/config.go
├── db/
│   ├── migrations/000001_create_users_table.{up,down}.sql
│   └── sqlc/                      # query.sql + generated Go
├── internal/
│   ├── handler/   service/   routes/
│   └── middleware/ models/   logger/
├── frontend/index.html
├── screenshots/                   # README screenshots (see screenshots/README.md)
├── sqlc.yaml  Dockerfile  docker-compose.yml
├── .env.example  .gitignore  README.md
```

## Configuration

All config comes from environment variables (see `.env.example`):

| Variable      | Default        | Description                |
|---------------|----------------|----------------------------|
| `APP_PORT`    | `3000`         | HTTP listen port           |
| `APP_ENV`     | `development`  | `development`/`production` (controls log format) |
| `LOG_LEVEL`   | `info`         | `debug`/`info`/`warn`/`error` |
| `DB_HOST`     | `localhost`    | MySQL host                 |
| `DB_PORT`     | `3306`         | MySQL port                 |
| `DB_USER`     | `root`         | MySQL user                 |
| `DB_PASSWORD` | _(required)_   | MySQL password — no default; must be set in the environment / `.env` |
| `DB_NAME`     | `ainyx_users`  | Database name              |

## Assignment Requirements Coverage

A checklist mapping each assignment requirement to where it is implemented, so it
can be verified at a glance.

| # | Requirement                                  | Status | Where / How                                                                                  |
|---|----------------------------------------------|:------:|----------------------------------------------------------------------------------------------|
| 1 | REST API with **GoFiber**                    |   ✅   | `cmd/server/main.go`, `internal/routes`, `internal/handler`                                   |
| 2 | **MySQL** as the database                    |   ✅   | `db/migrations`, `docker-compose.yml` (MySQL 8.0)                                             |
| 3 | **SQLC** for type-safe queries               |   ✅   | `db/sqlc/query.sql` → generated `query.sql.go`, `models.go`, `querier.go`; config in `sqlc.yaml` |
| 4 | **CRUD** on users (name + dob)               |   ✅   | `POST/GET/PUT/DELETE /users` in `internal/handler/user_handler.go`                            |
| 5 | **Age computed** from dob (not stored)       |   ✅   | `internal/service/age.go` (`CalculateAge`), applied in `toUserWithAge`                        |
| 6 | **DOB validation** (format / range / age)    |   ✅   | `internal/service/user_service.go` (`parseDOB`) + custom `dateonly` validator in the handler |
| 7 | **Pagination** for list endpoint             |   ✅   | `GET /users?page=&limit=`; `ListUsers` + `CountUsers` queries; envelope in `models`          |
| 8 | **Structured logging** with **Uber Zap**     |   ✅   | `internal/logger/logger.go`, request logging in `internal/middleware`                         |
| 9 | **Stack traces on errors** in logs           |   ✅   | `zap.AddStacktrace(zapcore.ErrorLevel)` in `logger.go`                                        |
| 10| **Request ID** middleware                    |   ✅   | `internal/middleware/middleware.go` (`RequestID`, `X-Request-ID`)                             |
| 11| **Centralized error handling** (one format)  |   ✅   | `ErrorHandler` middleware → `{ "error", "request_id" }` envelope                              |
| 12| **DB connection pooling**                    |   ✅   | `connectDB` in `main.go` (`SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`)           |
| 13| **Migrations** (golang-migrate format)       |   ✅   | `db/migrations/000001_create_users_table.{up,down}.sql`                                       |
| 14| **Dockerfile** (multi-stage build)           |   ✅   | `Dockerfile` (builder + minimal non-root Alpine runtime)                                      |
| 15| **docker-compose** (app + DB)                |   ✅   | `docker-compose.yml` (health-gated startup order)                                             |
| 16| **Unit tests**                               |   ✅   | `internal/service/age_test.go`, `dob_test.go` — run with `go test ./... -v`                   |
| 17| **Health check** endpoint                    |   ✅   | `GET /health` (DB ping) in `internal/handler/health_handler.go`                               |
| 18| **Graceful shutdown**                        |   ✅   | `SIGINT`/`SIGTERM` handling + `ShutdownWithContext` in `main.go`                              |
| 19| **Config via environment variables**         |   ✅   | `config/config.go`, `.env.example`                                                            |
| 20| **Frontend / demo UI**                       |   ✅   | `frontend/index.html` (zero-dependency, live-updating console)                               |

## Author

**Vamsi Kathi** | vamsikathi16@gmail.com |
[LinkedIn](https://linkedin.com/in/vamsi-kathi) |
[GitHub](https://github.com/Vamsi-Kathi)

## License

MIT
