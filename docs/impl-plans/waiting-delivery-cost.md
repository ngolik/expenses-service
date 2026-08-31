# Implementation Plan: Record a wait cost tied to a delivery

## Summary

- **Goal**: Add a new creation path that requires delivery id + auth-service-validated user id + amount, and a read-by-id path for the created record, without touching the existing `/expenses/rest/add` or `/expenses/rest/all` behavior.
- **In scope**: `model.Expense.DeliveryID` field; `authclient` package (auth-service HTTP client behind an interface); `service.AddWaitingDeliveryCost` / `service.GetWaitingDeliveryCost`; `POST /expenses/rest/waiting-cost` and `GET /expenses/rest/waiting-cost/:id` handlers + routes; unit tests for all of the above.
- **Out of scope**: `arrival-service` changes, remark/text field, list-all endpoint, live existence-check against `arrival-service`, any change to `AddExpenseHandler`/`GetExpensesHandler`/`AddExpense`/`GetAllExpenses`.
- **Architecture input**: `docs/architecture/waiting-delivery-cost.md`

## Touch map

| Area | Why |
| --- | --- |
| `model/expense.go` | Add `DeliveryID int` field to `Expense` |
| `authclient/client.go` (new) | `UserExists(userID int) (bool, error)` calling `GET {baseURL}/api/users/{id}` on auth-service |
| `authclient/client_test.go` (new) | Cover 200/404/other-status/network-error/env-var-default behavior using `httptest.Server` |
| `service/waiting_delivery_cost.go` (new) | `UserValidator`/`ExpenseRepository` interfaces, `ValidationError`/`UpstreamError` types, `AddWaitingDeliveryCost`, `GetWaitingDeliveryCost`, default GORM-backed repository |
| `service/waiting_delivery_cost_test.go` (new) | Table-driven tests against a fake repository + fake validator: all-fields-present success, each missing-field rejection, unknown-user rejection, validator-error → upstream error, repository-error passthrough, get-found, get-not-found |
| `api/waiting_delivery_cost.go` (new) | `AddWaitingDeliveryCostHandler`, `GetWaitingDeliveryCostHandler`, package-level default validator/repository vars |
| `api/waiting_delivery_cost_test.go` (new) | `httptest` + `gin.TestMode` tests against the handlers with fakes substituted for the package vars: 200 create, 400 per validation case, 502 upstream, 200 get, 404 get |
| `api/api.go` | Register the two new routes in the existing `/expenses/rest` group |

No changes to `api/expense.go`, `service/expense.go`, `database/config.go`, `main.go`.

## Steps

### Step 1 — Model: add `DeliveryID`

- **Outcome**: `Expense` carries an optional-at-the-DB-level delivery id; `AutoMigrate` will add the column on next startup.
- **Approach**: Add `DeliveryID int` to the `Expense` struct in `model/expense.go`, with a short comment explaining the zero-value semantics (unset for the general `/add` path).
- **Tests**: None standalone — exercised via Step 3/5 tests.
- **Done when**: Struct compiles; no other file needs to change for this step alone (no manual migration file — `AutoMigrate` handles it).

### Step 2 — `authclient`: isolate the auth-service call

- **Outcome**: A small, mockable client that answers "does this user id exist in auth-service?"
- **Approach**: New package `authclient`. `HTTPUserValidator{BaseURL string; HTTPClient *http.Client}` with `UserExists(userID int) (bool, error)`: GET `{BaseURL}/api/users/{id}`; 200 → `true, nil`; 404 → `false, nil`; anything else (including a transport error) → `false, err`. `NewHTTPUserValidator()` reads `AUTH_SERVICE_BASE_URL`, defaulting to `http://localhost:8081` if unset. No new dependency — stdlib `net/http` only.
- **Tests**: `authclient/client_test.go` using `httptest.NewServer` to stub 200/404/500 responses, plus a case pointing at a closed port to exercise the transport-error branch, plus a test that `NewHTTPUserValidator` picks up `AUTH_SERVICE_BASE_URL` when set and falls back to the default when unset.
- **Done when**: All `authclient` tests pass in isolation (no real auth-service needed).

### Step 3 — `service`: validation + persistence for the new path

- **Outcome**: A function that enforces "delivery id, validated user id, amount all present" and only then persists, plus a function to read one record back.
- **Approach**: New file `service/waiting_delivery_cost.go`. Define `UserValidator interface { UserExists(userID int) (bool, error) }` and `ExpenseRepository interface { Create(*model.Expense) error; FindByID(id uint) (*model.Expense, error) }` (interfaces owned by `service`, the consumer — `authclient.HTTPUserValidator` and a GORM-backed repository satisfy them structurally, no import from `service` back to `authclient`). Add `ValidationError` (→ 400 at the handler) and `UpstreamError` (→ 502 at the handler) error types. `AddWaitingDeliveryCost(expense model.Expense, validator UserValidator, repo ExpenseRepository) error`: reject with `ValidationError` if `DeliveryID == 0`, `UserID == 0`, or `Amount == 0`; call `validator.UserExists(expense.UserID)` — a transport/unexpected-status error becomes `UpstreamError`, a clean "not found" becomes `ValidationError`; otherwise `repo.Create(&expense)`, returning its error as-is (mapped to 500 by the handler). `GetWaitingDeliveryCost(id uint, repo ExpenseRepository) (*model.Expense, error)`: `repo.FindByID(id)`. Also add `DefaultExpenseRepository ExpenseRepository`, a small GORM-backed implementation using `database.DB`, for production wiring.
- **Tests**: `service/waiting_delivery_cost_test.go`, table-driven, using an in-memory fake `ExpenseRepository` (map-backed) and a fake `UserValidator` (configurable exists/error). Cases: happy path (record created, fields round-trip); missing delivery id; missing user id; missing amount; validator reports user does not exist; validator returns a transport error (→ `UpstreamError`, nothing persisted); repository `Create` error surfaces unchanged; `GetWaitingDeliveryCost` found; `GetWaitingDeliveryCost` not found.
- **Done when**: All cases pass without touching `database.DB` or any network call.

### Step 4 — `api`: handlers + routes

- **Outcome**: `POST /expenses/rest/waiting-cost` and `GET /expenses/rest/waiting-cost/:id` are reachable and return the right status codes.
- **Approach**: New file `api/waiting_delivery_cost.go`. Package-level vars `waitingDeliveryCostValidator service.UserValidator = authclient.NewHTTPUserValidator()` and `waitingDeliveryCostRepo service.ExpenseRepository = service.DefaultExpenseRepository` — real implementations by default, overridable in same-package tests. `AddWaitingDeliveryCostHandler`: `c.BindJSON(&model.Expense{})` (400 + `gin.H{"error": ...}` on bind failure, mirroring `AddExpenseHandler`), then `service.AddWaitingDeliveryCost(expense, waitingDeliveryCostValidator, waitingDeliveryCostRepo)`; `errors.As` against `*service.ValidationError` → 400, `*service.UpstreamError` → 502, anything else → 500; success → 200 + `gin.H{"message": ...}`. `GetWaitingDeliveryCostHandler`: parse `:id` (400 on non-numeric), `service.GetWaitingDeliveryCost`; `gorm.ErrRecordNotFound` → 404, other error → 500, success → 200 + a small response struct `{id, deliveryId, userId, amount}`. Register both routes in `api.SetupRoutes` alongside the existing two, same group, no reordering of the existing lines.
- **Tests**: `api/waiting_delivery_cost_test.go`, `gin.SetMode(gin.TestMode)` + `httptest`, substituting `waitingDeliveryCostValidator`/`waitingDeliveryCostRepo` package vars with fakes (no real DB, no real auth-service). Cases: 200 create; 400 for each missing field; 400 for unknown user; 502 when the validator errors; 400 for malformed JSON body (bind failure); 200 get with correct body shape; 404 get for missing id; 400 get for non-numeric id.
- **Done when**: All cases pass; `AddExpenseHandler`/`GetExpensesHandler` and their existing routes are untouched (verified by re-reading `api/api.go` and `api/expense.go` diffs — should show only additive lines).

### Step 5 — Full-repo verification

- **Outcome**: Confirm nothing else broke.
- **Approach**: From ROOT, run `go build ./...` then `go test ./...`.
- **Tests**: All of the above, run together.
- **Done when**: Both commands exit 0. **[Caveat]** — if the Go toolchain is unavailable in the execution environment, this step is reported as "not run" rather than assumed to pass.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| `auth-service`'s `application.yml`/`application-dev.yml` set `server.servlet.context-path: /auth/`, which would prefix every route with `/auth`, conflicting with the spec's confirmed `/api/users/{id}` (no prefix) | `AUTH_SERVICE_BASE_URL` is fully configurable per environment; documented as `[INFERRED — please validate]` in the architecture doc rather than silently guessed at. |
| Auth-service failure modes beyond "404 = unknown user" aren't specified by the spec (network error, 500, malformed response) | Treated as a distinct `UpstreamError` → 502, so it's neither silently accepted nor conflated with a 400 validation failure. Documented as `[INFERRED — please validate]`. |
| Reusing the `Expense` table for wait-cost records means `GET /expenses/rest/all` will now also return wait-cost rows (with `DeliveryID` populated) alongside plain expenses | Acceptable per spec — no list-all endpoint is required for wait-cost records, and the spec doesn't require *excluding* them from the existing all-expenses list either. Not treated as a behavior change to `/all`'s contract (same handler, same query, same response shape) — it will simply include more rows over time, no different than expenses added via `/add` growing that list today. If this is undesired, filtering `/all` to exclude `DeliveryID != 0` rows is a follow-up, not implied by any AC here. |
| No Go toolchain available in this environment to run `go build`/`go test` | Reported plainly as not run, not claimed as passing; code is written to compile against the confirmed `go.mod` dependency set (no new external dependency added) to minimize the chance of a build break. |

## Open questions

- None blocking — two `[INFERRED — please validate]` items above (auth-service base path re: context-path, and upstream-failure status code) are carried into the architecture doc and should be confirmed once a running `auth-service` instance is available to test against.
