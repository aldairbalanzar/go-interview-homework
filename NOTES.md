# Notes

Reviewed by: AB

## 1. Seed bug
The INSERT into `tasks` quoted column names with backticks (`user_id`, `title`, …). That’s MySQL identifier quoting. Postgres treats backticks as illegal syntax, so the first seed run failed on the task insert (users had already been written in the same transaction, which rolled back).

I found it by running `go run ./cmd/seed` after Postgres was up and reading the driver error on that statement. Removing the backticks matches `db/schema.sql`. Re-runs truncate `task_tags`, `tasks`, and `users` first so the dataset stays deterministic.

A missing `go.sum` also blocked the first compile; `go get` / `go mod tidy` filled that in. That was environment/modules, not the intended SQL bug.

## 2. Library / structure
I used **gqlgen** because it generates typed stubs from a `.graphqls` schema, which is a good fit for a fixed required API and less stringly-typed than `graphql-go`. Schema lives in `graph/schema.graphqls`; resolvers in `graph/`; generated code is left generated.

Layout:
- `cmd/seed` — seeder
- `cmd/server` — HTTP + GraphQL on `:8080` at `/graphql`
- `internal/store` — Postgres access (`database/sql` + `pgx` stdlib)
- `graph` — GraphQL mapping and resolvers only

Keep SQL out of resolvers so the API layer stays a thin translation (IDs, `PENDING` vs `pending`, etc.).

## 3. New field
I picked **`taskCount` on `User`**. `dueDate` and `description` were already in the Part 2 schema; `taskCount` is a computed field that has to go schema → `COUNT(*)` in Postgres → UI, which is the cleaner three-layer cut. Optional `status` is on the schema; the board shows the unfiltered total.

## 4. Tradeoffs / skipped
- Nested fields hit the DB once per user. The board query loads users, then runs a COUNT and a task list for each person (1 + N + N queries). That’s fine for three seeded users but with more I’d batch with `GROUP BY user_id` / `WHERE user_id IN (...)` so one request does a handful of SQL statements instead of one per user.
- Mutations don’t validate tag uniqueness beyond the PK; invalid `dueDate` format returns an error.

## 5. How to run (fresh clone)
Needs Go 1.25+, Docker, and a browser. From the repo root:

```bash
docker compose up -d
go run ./cmd/seed
go run ./cmd/server
```

GraphQL: `http://localhost:8080/graphql`  
Open `http://localhost:8081/`.

Default DB URL matches compose: `postgres://admin:todo@localhost:5432/homework?sslmode=disable`. Override with `DATABASE_URL` if needed.

I used Cursor while building this.

Reviewed by: AB
