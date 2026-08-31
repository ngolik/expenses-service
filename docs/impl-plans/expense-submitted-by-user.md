# Implementation Plan: Expense submitted-by-user

## Summary

- **Goal:** Let an operator record and later retrieve which existing system
  user submitted an expense.
- **In scope:** Optional `UserID` on create, validated when sent; new
  get-by-id endpoint that echoes stored `UserID`; existing create/list
  behavior unchanged for callers that omit `UserID`.
- **Out of scope:** New auth/registration, auth-service calls, UI, Eureka
  config, backfill, framework upgrades.
- **Architecture input:** `docs/architecture/expense-submitted-by-user.md`

## Touch map

| Area | Why |
| --- | --- |
| `database/config.go` | Auto-migrate `model.User` so the users table exists |
| `service/expense.go` | Validate `UserID` against `users` table on create via a `userExpenseRepository` seam; add `GetExpenseById` |
| `api/expense.go` | Map "unknown user" to `400`; add `GetExpenseByIdHandler` |
| `api/api.go` | Register `GET /expenses/rest/:id` |
| `service/expense_test.go` (new) | Unit tests against an in-memory fake repository: happy path, omitted field, unknown-user validation, get-by-id echo, get-by-id not-found |

`model/expense.go` is unchanged (field type/tag already fit the "0 = omitted"
decision from architecture).

## Steps

### Step 1 — Migrate the users table

- **Outcome:** `model.User` rows can be created/queried; no behavior change
  to existing expense flows.
- **Approach:** Add `&model.User{}` to the `DB.AutoMigrate(...)` call in
  `database.MigrateDatabase`.
- **Tests:** Covered indirectly by Step 3 tests (they insert user rows).
- **Done when:** `go build ./...` succeeds; migration call includes both models.

### Step 2 — Add "unknown user" error + service validation on create

- **Outcome:** `service.AddExpense` rejects a non-zero `UserID` that has no
  matching row in `users`; `UserID == 0` (omitted) still passes through
  unchanged.
- **Approach:** Add a sentinel error (e.g. `service.ErrUnknownUser`) in
  `service/expense.go`. In `AddExpense`, when `expense.UserID != 0`, run a
  `database.DB.First(&model.User{}, expense.UserID)` lookup; on
  `gorm.ErrRecordNotFound` return `ErrUnknownUser`; on other DB errors
  return the original error (existing `500` path unchanged).
- **Tests:** Unit tests using an in-memory sqlite `gorm.DB` swapped into
  `database.DB` for the test:
  - create with `UserID` referencing a seeded user → success, row persisted
    with that `UserID`.
  - create with `UserID: 0` → success (unchanged today's behavior).
  - create with a non-existent `UserID` → `ErrUnknownUser` returned, no
    expense row persisted.
- **Done when:** the three cases above pass via `go test ./...`.

### Step 3 — Map validation error to 400 in the API layer

- **Outcome:** `POST /expenses/rest/add` returns `400` with a clear message
  when `service.AddExpense` returns `ErrUnknownUser`; unrelated DB errors
  keep returning `500` as today.
- **Approach:** In `AddExpenseHandler`, branch on
  `errors.Is(err, service.ErrUnknownUser)` before the existing `500` path.
- **Tests:** Extend the service-level tests (Step 2) — API-level coverage is
  optional since the handler is a thin branch; if added, a `gin` test using
  `httptest` with the same in-memory DB.
- **Done when:** unknown-user case returns `400`; omitted/valid cases still
  return `200` as today.

### Step 4 — Add get-by-id endpoint

- **Outcome:** `GET /expenses/rest/:id` returns the persisted expense
  (including the `UserID` exactly as stored) or `404` when not found.
- **Approach:** Add `service.GetExpenseById(id uint) (model.Expense, error)`
  using `database.DB.First`. Add `GetExpenseByIdHandler` in `api/expense.go`
  parsing `:id`, calling the service, returning `404` on
  `gorm.ErrRecordNotFound` and `200` + the expense otherwise. Register the
  route in `api/api.go`: `expensesGroup.GET("/:id", GetExpenseByIdHandler)`.
- **Tests:** Unit test: create an expense with a `UserID`, then
  `GetExpenseById` returns the same `UserID` unchanged (not derived from any
  auth context, per acceptance criteria).
- **Done when:** get-by-id test passes; existing `GET /expenses/rest/all`
  still registered and unaffected (route order: `/all` is a distinct static
  path from `/:id`, no conflict in gin).

### Step 5 — Wire test seam (revised — see Decision Drift below)

- **Outcome:** Tests run without a live Postgres instance (CI has none —
  `go.yml` just runs `go test -v ./...`) and without adding a new module
  dependency.
- **Approach:** `AddExpense` / `GetExpenseById` reach storage through a small
  `userExpenseRepository` interface (`service/expense.go`), with a
  GORM-backed default (`gormRepository`, production behavior unchanged) and
  an in-memory fake (`fakeRepository`, `service/expense_test.go`) swapped
  into the package-level `Repo` var for tests.
- **Tests:** n/a (infrastructure for Step 2/4 tests).
- **Done when:** `go test -v ./...` passes without any live database.

> **Decision Drift:** the original step proposed adding `modernc.org/sqlite`
> as a test-only dependency. The engineering environment had no Go
> toolchain and no cached modules available, so a new dependency could not
> be added with a verified `go.sum` (hand-editing checksums would risk a
> build no one could trust). Switched to a dependency-free repository-seam
> + in-memory fake, which gives the same test coverage without touching
> `go.mod`/`go.sum` at all.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Repository seam adds indirection not in the original architecture doc | Minimal (one interface, one prod impl, one test fake); production behavior via `gormRepository` is identical to the original direct `database.DB` calls |
| Global `Repo` var swap between tests could leak state if tests run in parallel | Tests run sequentially (no `t.Parallel()`); each test restores the previous `Repo` via `t.Cleanup` |
| `model.User` has no seed data path (out of scope) | Tests seed rows directly via GORM; documented as a known gap in architecture doc |

## Open questions

- None blocking — architecture decisions already confirmed at G1.
