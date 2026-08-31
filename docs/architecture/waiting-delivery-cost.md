# Architecture: Record a wait cost tied to a delivery

## 1. Scope and constraints

- **Problem**: `expenses-service` has no way to create a mandatory money amount tied to a delivery id and an identified operator. The existing `POST /expenses/rest/add` accepts any `Expense` with an optional `UserID` and no delivery concept at all.
- **Goals**: A new, additional creation path that requires all three of delivery id, an identified (auth-service-validated) operator, and an amount — reject if any is missing/invalid. A new read-by-id path returning delivery id, user id, and amount.
- **Non-goals**: The warehouse-facing "mark delivery as waiting + optional remark" half (owned by `arrival-service`, out of this repo). No remark/text field here. No list-all endpoint for wait-cost records. No live call to `arrival-service` to confirm the delivery id exists — presence of the id is enough per the approved spec. No change to `POST /expenses/rest/add` or `GET /expenses/rest/all` behavior.
- **Constraints**: Go 1.18, Gin, GORM, Postgres — confirmed from `go.mod`/`database/config.go`. No new external dependency added for the HTTP call to `auth-service` (stdlib `net/http` is sufficient). No new dependency added for testing either — see §4 on the repository interface vs. a sqlite driver.
- **Sources used**: `docs/specifications/waiting-delivery-cost.md` (scratch-ai-skills-pilot, read-only reference); `auth-service/src/main/java/.../controller/UserController.java` and `dto/UserDTO.java` (read-only reference); this repo's `model/expense.go`, `api/api.go`, `api/expense.go`, `service/expense.go`, `database/config.go`.

## 2. Existing shape to respect

- `api.SetupRoutes` groups everything under `/expenses/rest` and wires two handlers: `AddExpenseHandler` (binds JSON straight into `model.Expense`, calls `service.AddExpense`, no validation) and `GetExpensesHandler` (`service.GetAllExpenses`, no filtering).
- `model.Expense` embeds `gorm.Model` (so it already has `ID`/timestamps) plus `Description`, `Amount`, `Category`, `Date`, `UserID`, `Healthy`. `UserID` exists today but is fully unvalidated/unused.
- `database.MigrateDatabase()` runs `DB.AutoMigrate(&model.Expense{})` on every startup — any new field added directly to `Expense` is picked up automatically, no manual migration step needed.
- There is no existing pattern in this repo for calling another service over HTTP, and no existing `_test.go` files — this change introduces both the first outbound HTTP dependency and the first tests.

## 3. Recommended change

| Component | Responsibility | Notes |
| --- | --- | --- |
| `model.Expense` | Add `DeliveryID int` field | Reuses the existing `Expense`/`expenses` table rather than introducing a second table. Zero value = "not linked to a delivery", which is exactly what the untouched `/add` path continues to produce. |
| `authclient` (new package) | Isolate the outbound call to `auth-service` | `HTTPUserValidator` calls `GET {baseURL}/auth/api/users/{id}`; base URL (host:port only) from `AUTH_SERVICE_BASE_URL` env var, default `http://localhost:8081`. Structurally implements the `service.UserValidator` interface — `authclient` has no dependency on `service`, so it can be swapped for a fake in tests without importing HTTP machinery. |
| `service` (new file `waiting_delivery_cost.go`) | Business rules for the new path | Defines `UserValidator` and `ExpenseRepository` interfaces (consumer-defined, dependency-inversion style) so the auth-service call and the DB write are both mockable. `AddWaitingDeliveryCost` validates delivery id / user id / amount presence, calls the validator, then persists via the repository. `GetWaitingDeliveryCost` reads by id via the repository. Existing `AddExpense`/`GetAllExpenses` in `expense.go` are untouched. |
| `api` (new file `waiting_delivery_cost.go`) | New HTTP surface, composition root for the new dependencies | `AddWaitingDeliveryCostHandler` (`POST /expenses/rest/waiting-cost`) and `GetWaitingDeliveryCostHandler` (`GET /expenses/rest/waiting-cost/:id`). Wires the real `authclient.HTTPUserValidator` and the real GORM-backed repository as package-level vars that tests can override with fakes. Existing `AddExpenseHandler`/`GetExpensesHandler` untouched. |
| `api.SetupRoutes` | Route registration | Adds the two new routes under the existing `/expenses/rest` group; the two existing routes are unchanged (same path, same handler, same order). |
| `database` | Migration | No code change — `AutoMigrate(&model.Expense{})` already covers the new `DeliveryID` column. |

- Happy path (create): handler binds JSON → `service.AddWaitingDeliveryCost` checks delivery id / user id / amount are non-zero → calls `UserValidator.UserExists(userID)` → on `true`, persists via `ExpenseRepository.Create` → 200 with `{id, deliveryId, userId, amount}` (the generated `id` included so the caller can retrieve the record afterwards — `AddWaitingDeliveryCost` takes `*model.Expense` rather than a value specifically so `repo.Create`'s id assignment is visible to the handler after the call returns). **Fixed post-review**: the original implementation returned only `{"message": "..."}`, with no way to retrieve the record afterwards — flagged in `docs/qa/waiting-delivery-cost-coverage.md` (AC7) and the PR body; corrected in this pass.
- Validation failure (missing field, or auth-service reports the user id doesn't exist): `service.ValidationError` → handler returns 400 with `gin.H{"error": ...}`, matching the existing error-body shape.
- Auth-service unreachable / unexpected response: distinguished as an upstream failure (`service.UpstreamError`) → handler returns 502, not 400 (it isn't the caller's input that's wrong) and not silently treated as success.
- Persistence failure (DB error after all validation passed): plain `error` → handler returns 500, consistent with `AddExpenseHandler`'s existing DB-error handling.
- Read: handler parses `:id` → `service.GetWaitingDeliveryCost` → 404 if not found (`gorm.ErrRecordNotFound`), 500 for any other DB error, 200 with `{id, deliveryId, userId, amount}` otherwise.

## 4. Key decisions and risks

| Decision | Options considered | Choice | Rationale |
| --- | --- | --- | --- |
| Model shape for the delivery link | New table/model vs. new field on `Expense` | New field (`DeliveryID int`) on the existing `Expense`/table | The wait-cost record *is* an expense (money amount) with two extra facts attached (delivery id, mandatory user). A second table would duplicate `Amount`/timestamps/id machinery for no behavioral gain, and the spec's read AC only needs delivery id + user id + amount, which the existing row already carries once the field is added. |
| Isolating the `auth-service` call | Call `net/http` directly from the handler vs. a small client package behind an interface | `authclient` package + `service.UserValidator` interface, interface defined in `service` (consumer), implementation in `authclient` | Matches the task's requirement to keep the call mockable/stubbable without a live `auth-service`. Defining the interface at the consumer (not in `authclient`) avoids `service` depending on HTTP machinery and avoids any import cycle. |
| Test doubles for persistence | GORM against sqlite in-memory vs. a fake `ExpenseRepository` | Fake repository interface | Adding a sqlite GORM driver is a new dependency whose `go.sum` entries can't be verified in this environment (no Go toolchain available to run `go mod tidy`/`go build`). A fake in-memory repository needs no new dependency and directly satisfies the "your call, document it" latitude given for this step. Existing `AddExpense`/`GetAllExpenses` keep using `database.DB` directly, unchanged — only the new path goes through the interface. |
| Auth-service base path | The cross-repo contract memo (written before this implementation) asserted the direct route was `/api/users/{id}` with no `/auth` prefix — but `auth-service`'s own `application.yml`/`application-dev.yml` both set `server.servlet.context-path: /auth/`, confirmed by reading both files during this implementation | **Corrected during implementation, not the memo's assumption**: path construction uses `/auth/api/users/{id}`; `AUTH_SERVICE_BASE_URL` stays host:port only (default `http://localhost:8081`) | The controller is mapped at `/api/users` (`UserController.java`), but Spring's `server.servlet.context-path` prefixes *every* route the app serves — the memo's claim that this was a gateway-only prefix was checked against the controller annotation only, not `application.yml`. Fixed here; the cross-repo contract memo in the specification should be corrected to match (Decision Drift). |
| Unreachable/unexpected-status auth-service response | Treat as validation failure (400) vs. a distinct upstream-failure status | Distinct `UpstreamError` → 502 | An auth-service outage is not the caller's fault; conflating it with "you sent bad input" (400) would be misleading. `[INFERRED — please validate]` — the spec only says "treat unknown/404 as rejection," it doesn't specify behavior for other auth-service failures; 502 is this repo's inference, not an explicit AC. |

## 5. Implementation handoff

- Touch areas: `model/expense.go` (add field), `authclient/client.go` (new), `service/waiting_delivery_cost.go` (new), `api/waiting_delivery_cost.go` (new), `api/api.go` (register two routes). No change to `api/expense.go`, `service/expense.go`'s existing functions, or `database/config.go`.
- Tests: `authclient/client_test.go`, `service/waiting_delivery_cost_test.go`, `api/waiting_delivery_cost_test.go` — first `_test.go` files in this repo, standard `testing` package, table-driven where the shape fits.
- Config surface added: `AUTH_SERVICE_BASE_URL` env var (default `http://localhost:8081`).
- Branch: `feature/waiting-delivery-cost` (already created, cut from `main`).
