# Architecture: Expense submitted-by-user

## 1. Scope and constraints

- **Problem**: An expense record cannot currently identify which system user
  submitted it. Operators need to set and later retrieve that identity.
- **Goals**: Optional `UserID` on create; validated against known users;
  echoed back verbatim on get-by-id.
- **Non-goals**: New auth/registration, changing how users are stored or how
  login works, UI, Eureka config, historical backfill, framework upgrades.
- **Constraints**: Stay on Go/Gin/GORM. auth-service (Java/Spring) is
  consumed as-is — no change in this story. Reuse existing user identity, no
  second user table.
- **Sources used**: `docs/specifications/expense-submitted-by-user.md`;
  code areas `model/`, `api/`, `service/`, `database/`.

## 2. Existing shape to respect

- `model.Expense` (GORM model) already has an untyped `UserID int` field —
  never populated or validated today.
- `model.User` already exists in this repo (`ID`, `Name`, `UserName`,
  `Password`) but is **not** currently `AutoMigrate`d or read/written
  anywhere — it is the local reflection of system-user identity.
  `[INFERRED — please validate]`: this struct is the "existing user catalog"
  the specification refers to (not a live call to auth-service), because
  auth-service integration is explicitly out of scope and no HTTP client to
  it exists in this repo.
- `api.AddExpenseHandler` binds JSON straight into `model.Expense` and calls
  `service.AddExpense` with no validation.
- `api.GetExpensesHandler` only lists all expenses; there is **no**
  get-by-id endpoint today, but the acceptance criteria require one
  (`Get-by-id returns the expense and echoes the stored UserID`).
- `database.MigrateDatabase` only auto-migrates `model.Expense`.

## 3. Recommended change

| Component | Responsibility | Notes |
| --- | --- | --- |
| `database` | Auto-migrate `model.User` alongside `model.Expense` | Table must exist for FK-style validation and for tests to seed users |
| `service` (expense) | Validate `UserID` against the `users` table when non-zero, before create; reject with a typed "unknown user" error otherwise pass through | Keeps validation next to the existing `AddExpense`/`GetAllExpenses` functions rather than a new layer |
| `api` (expense) | Map the new service validation error to `400`; add `GetExpenseByIdHandler` (`GET /expenses/rest/:id`) that loads by primary key and returns the stored `Expense` as-is (including `UserID`) | No new auth/identity parsing — the id returned is exactly what GORM stored, not derived from any auth context |

- **Happy path**: caller sends `UserID` (or omits it) on create → service
  checks `UserID == 0` (omitted, always allowed) or `UserID` exists in
  `users` → persists → get-by-id later returns the same `UserID` verbatim.
- **Failure**: `UserID` set but no matching user row → `400` with a clear
  message; existing `500`-on-DB-error behavior is unchanged.
- **Data classes**: `UserID` is a non-sensitive reference (int), no PII
  travels through this change since `model.User.Password` is never read or
  exposed by these endpoints.
- **Authn/authz**: unchanged — these endpoints remain unauthenticated in
  this repo today; this story does not add authorization checks (non-goal:
  "new authentication or registration flows").

## 4. Key decisions and risks

| Decision | Options considered | Choice | Rationale |
| --- | --- | --- | --- |
| Where user identity lives | (a) call auth-service over HTTP; (b) reuse local `model.User` table | (b) reuse local `model.User` table | Spec explicitly excludes new auth flows / Eureka config and says "do not duplicate a second user table" — (a) would require a new client + service discovery; (b) already exists and satisfies "reuse existing user identity" |
| Optional-field representation | (a) pointer `*int`; (b) keep `int`, treat `0` as "omitted" | (b) keep `int`, `0` = omitted | No behavior change to the existing field type/tag; GORM PKs start at 1 so `0` is never a real user id — avoids a wire-format change for existing callers (Unchanged contracts: "existing create/list keep working for callers that omit `UserID`") |
| Validation error shape | (a) generic `500`; (b) `400` with clear message | (b) `400` | Acceptance criteria: "Unknown user identity is rejected (4xx)" |

- **Risk**: `model.User` has no rows today; tests and any manual verification
  must seed at least one user row. `[MISSING — input needed]`: no seed/admin
  flow exists to create users through this API (non-goal — out of scope),
  so tests will insert user rows directly via GORM in test setup.

## 5. Implementation handoff

- Touch areas: `model/expense.go` (no field-type change, doc comment only),
  `database/config.go` (migrate `model.User`), `service/expense.go`
  (validation + `GetExpenseById`), `api/expense.go` (map validation error,
  add get-by-id handler), `api/api.go` (register `GET /expenses/rest/:id`).
- Tests: happy path (create with valid `UserID`, get-by-id echoes it),
  omitted-field path (create without `UserID`, still `200`), unknown-user
  path (create with non-existent `UserID`, `400`).
- Suggested branch: `feature/expense-submitted-by-user` (already checked out).
