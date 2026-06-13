# ainyx-user-api

A production-quality REST API for managing users, built with **GoFiber**, **MySQL**, and **SQLC**. Each user has a name and date of birth; the API computes the user's age on read.

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

## Running Locally (without Docker)

Prerequisites: Go 1.26+, a running MySQL 8.0 with database `ainyx_users`.

1. Copy env and adjust if needed:
   ```bash
   cp .env.example .env
   ```
2. Apply the schema (any one of):
   - Run the SQL in `db/migrations/000001_create_users_table.up.sql` against your DB, **or**
   - Use golang-migrate:
     ```bash
     migrate -path db/migrations \
       -database "mysql://root:3499@tcp(localhost:3306)/ainyx_users" up
     ```
3. Run:
   ```bash
   go run ./cmd/server
   ```
   Server listens on `http://localhost:3000`.

## Running with Docker

Brings up MySQL (with health check) and the app together. The schema is auto-applied
on first boot via an init script mounted into the MySQL container.

```bash
docker compose up --build
```

- App: http://localhost:3000
- MySQL: localhost:3306

> **Note on Go version:** the assignment suggested `golang:1.22-alpine` for the
> final image, but this module targets Go 1.26, which cannot be compiled by 1.22.
> The Dockerfile therefore uses `golang:1.26-alpine` as the builder and a minimal
> `alpine:3.20` runtime image (smaller and more secure than shipping the full Go image).

## Frontend

A zero-dependency, single-file console lives at `frontend/index.html`. Open it in a
browser while the API is running (it defaults to `http://localhost:3000`) to create,
list, edit, and delete users.

## Code Generation (SQLC)

Queries live in `db/sqlc/query.sql`; the schema in `db/migrations`. Regenerate with:

```bash
sqlc generate
```

## Tests

The age calculation is fully unit-tested (born today, exact N years, pre/post
birthday, leap-year Feb 29, future-date clamping):

```bash
go test ./...
# verbose:
go test ./internal/service -v
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
│   ├── middleware/ models/   logger/
│   └── repository/
├── frontend/index.html
├── sqlc.yaml  Dockerfile  docker-compose.yml
├── .env  .env.example  .gitignore  README.md
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
| `DB_PASSWORD` | `3499`         | MySQL password             |
| `DB_NAME`     | `ainyx_users`  | Database name              |
