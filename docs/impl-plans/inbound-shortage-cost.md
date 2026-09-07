# Implementation Plan: Record a shortage amount tied to a delivery

## Summary

- **Goal**: Add a new creation path that requires delivery id + auth-service-validated user id + amount, marks the record as a shortage (distinct from a `waiting-delivery-cost` or `damaged-inbound-writeoff` record for the same delivery, spec AC1), and a read-by-id path for the created record (spec AC7) - without touching the existing `/expenses/rest/add`, `/expenses/rest/all`, `/expenses/rest/waiting-cost*`, or `/expenses/rest/damage-writeoff*` behavior.
- **In scope**: `model.Expense.IsShortage` field; `service.AddShortage` / `service.GetShortage` (reusing the existing `UserValidator`/`ExpenseRepository` interfaces and `ValidationError`/`UpstreamError` types from `service/waiting_delivery_cost.go`); `POST /expenses/rest/shortage` and `GET /expenses/rest/shortage/:id` handlers + routes; unit tests for all of the above, including a cross-story distinguishability check against both sibling record kinds.
- **Out of scope**: `arrival-service` changes (owns the warehouse-facing shortage fact + remark, per the spec's `## Repos` `arrival-service` paragraph), remark/text field on `Expense`, list-all endpoint (spec AC8), live existence-check against `arrival-service`, any change to `AddExpenseHandler`/`GetExpensesHandler`/`AddExpense`/`GetAllExpenses`/`AddWaitingDeliveryCostHandler`/`GetWaitingDeliveryCostHandler`/`AddWaitingDeliveryCost`/`GetWaitingDeliveryCost`/`AddDamageWriteOffHandler`/`GetDamageWriteOffHandler`/`AddDamageWriteOff`/`GetDamageWriteOff`.
- **Architecture input**: `docs/architecture/inbound-shortage-cost.md`

## Touch map

| Area | Why |
| --- | --- |
| `model/expense.go` | Add `IsShortage bool` field to `Expense` |
| `service/shortage.go` (new) | `AddShortage`, `GetShortage` - reuses `UserValidator`/`ExpenseRepository`/`ValidationError`/`UpstreamError`/`newValidationError`/`newUpstreamError` from `waiting_delivery_cost.go`, no redefinition |
| `service/shortage_test.go` (new) | Table-driven tests against the existing fake repository + fake validator: all-fields-present success (including `IsShortage == true` on the stored record), each missing-field rejection, unknown-user rejection, validator-error → upstream error, repository-error passthrough, get-found, get-not-found, and a dedicated distinguishability test creating a wait-cost, a damage-writeoff, and a shortage record for the same `DeliveryID` |
| `api/shortage.go` (new) | `AddShortageHandler`, `GetShortageHandler`, package-level default validator/repository vars |
| `api/shortage_test.go` (new) | `httptest` + `gin.TestMode` tests against the handlers with fakes substituted for the package vars: 200 create, 400 per validation case, 502 upstream, 500 db failure, 400 malformed body, 200 get, 404 get, 400 non-numeric id, 500 other repo error, and a handler-level distinguishability check |
| `api/api.go` | Register the two new routes in the existing `/expenses/rest` group |

No changes to `authclient/client.go`, `service/waiting_delivery_cost.go`, `service/waiting_delivery_cost_test.go`, `service/damage_writeoff.go`, `service/damage_writeoff_test.go`, `api/waiting_delivery_cost.go`, `api/waiting_delivery_cost_test.go`, `api/damage_writeoff.go`, `api/damage_writeoff_test.go`, `api/expense.go`, `service/expense.go`, `database/config.go`, `main.go`.

## Steps

### Step 1 - Model: add `IsShortage`

- **Outcome**: `Expense` carries a discriminator that stays `false` for every existing record (plain `/add`, `waiting-delivery-cost`, `damage-writeoff`) and is only ever `true` for a shortage record; `AutoMigrate` adds the column on next startup.
- **Approach**: Add `IsShortage bool` to the `Expense` struct in `model/expense.go`, with a comment explaining the discriminator purpose (mirrors `IsDamageWriteOff`'s comment style).
- **Tests**: None standalone - exercised via Step 2/3 tests.
- **Done when**: Struct compiles; no other file needs to change for this step alone.

### Step 2 - `service`: validation + persistence for the new path

- **Outcome**: A function that enforces "delivery id, validated user id, amount all present" (spec AC2, AC3, AC4) and marks the record as a shortage before persisting, plus a function to read one record back (spec AC7).
- **Approach**: New file `service/shortage.go`. Reuses `UserValidator`, `ExpenseRepository`, `ValidationError`, `UpstreamError`, `newValidationError`, `newUpstreamError` from `waiting_delivery_cost.go` (same package, no redefinition). `AddShortage(expense *model.Expense, validator UserValidator, repo ExpenseRepository) error`: reject with `ValidationError` if `DeliveryID == 0`, `UserID == 0`, or `Amount == 0`; call `validator.UserExists(expense.UserID)` - a transport/unexpected-status error becomes `UpstreamError`, a clean "not found" becomes `ValidationError`; otherwise set `expense.IsShortage = true` and call `repo.Create(expense)`, returning its error as-is. `GetShortage(id uint, repo ExpenseRepository) (*model.Expense, error)`: `repo.FindByID(id)`.
- **Tests**: `service/shortage_test.go`, table-driven, reusing the existing `fakeExpenseRepository`/`fakeUserValidator` from `waiting_delivery_cost_test.go` (same package). Cases: happy path (record created, `IsShortage == true`, generated id visible on the caller's pointer); missing delivery id; missing user id; missing amount; validator reports user does not exist; validator returns a transport error (→ `UpstreamError`, nothing persisted); repository `Create` error surfaces unchanged; `GetShortage` found; `GetShortage` not found. Plus a distinguishability test: creates a wait-cost record, a damage-writeoff record, and a shortage record for the same `DeliveryID` in one shared fake repository, fetches all three back, asserts `IsShortage` is `false` on the wait-cost and damage-writeoff ones and `true` on the shortage one (spec AC1).
- **Done when**: All cases pass without touching `database.DB` or any network call.

### Step 3 - `api`: handlers + routes

- **Outcome**: `POST /expenses/rest/shortage` and `GET /expenses/rest/shortage/:id` are reachable and return the right status codes.
- **Approach**: New file `api/shortage.go`, mirroring `api/waiting_delivery_cost.go`/`api/damage_writeoff.go` structure exactly. Package-level vars `shortageValidator service.UserValidator = authclient.NewHTTPUserValidator()` and `shortageRepo service.ExpenseRepository = service.DefaultExpenseRepository`. `AddShortageHandler`: `c.BindJSON(&model.Expense{})` (400 on bind failure), then `service.AddShortage(&expense, shortageValidator, shortageRepo)`; `errors.As` against `*service.ValidationError` → 400, `*service.UpstreamError` → 502, anything else → 500; success → 200 + `ShortageResponse{id, deliveryId, userId, amount}`. `GetShortageHandler`: parse `:id` (400 on non-numeric), `service.GetShortage`; `gorm.ErrRecordNotFound` → 404, other error → 500, success → 200 + the same response shape. Register both routes in `api.SetupRoutes` alongside the existing six, same group, no reordering of existing lines.
- **Tests**: `api/shortage_test.go`, reusing `setupRouterForTest`/`performRequest`/`fakeValidator`/`waitingCostFakeRepo` from `waiting_delivery_cost_test.go` (same package). Cases: 200 create with a non-zero `id` in the response body; 400 for each missing field; 400 for unknown user; 502 when the validator errors; 500 on db create failure; 400 for malformed JSON body; 200 get with correct body shape; 404 get for missing id; 400 get for non-numeric id; 500 get for other repository error. Plus a handler-level distinguishability test: creates a wait-cost record, a damage-writeoff record, and a shortage record for the same `DeliveryID` through the real handlers/routes against a shared fake repo, checks each response body's fields independently and that the discriminator is set correctly for each.
- **Done when**: All cases pass; `AddExpenseHandler`/`GetExpensesHandler`/`AddWaitingDeliveryCostHandler`/`GetWaitingDeliveryCostHandler`/`AddDamageWriteOffHandler`/`GetDamageWriteOffHandler` and their existing routes are untouched (verified by re-reading `api/api.go` diff - only additive lines).

### Step 4 - Full-repo verification

- **Outcome**: Confirm nothing else broke.
- **Approach**: From ROOT, run `go build ./...` then `go test ./...`.
- **Tests**: All of the above, run together.
- **Done when**: Both commands exit 0.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Reusing the `Expense` table for shortage records means `GET /expenses/rest/all` will now also return shortage rows (with `IsShortage` populated) alongside plain expenses, wait-cost rows, and damage-writeoff rows | Acceptable, matching the `waiting-delivery-cost`/`damaged-inbound-writeoff` precedent - no list-all endpoint is required for shortage records (spec AC8 explicitly puts a shortage-records list out of scope), and the spec doesn't require excluding them from the existing all-expenses list. Not treated as a behavior change to `/all`'s contract. |
| `IsShortage` not exposed in `ShortageResponse`/`WaitingDeliveryCostResponse`/`DamageWriteOffResponse` | Intentional (see architecture doc §4) - the route/response-type pair already discriminates for the caller; the flag is a storage-level implementation detail for AC1, not part of any existing read contract. If a future consumer needs the raw flag (e.g. a combined view across all record types), that is a new, separate read shape, not a change to any existing response. |
| This story's shortage amount and the siblings' wait-cost/damage-writeoff amounts live in the same table/columns, distinguished only by a boolean each | This is the approved AC1 mechanism per the specification's `## Repos` `expenses-service` paragraph (resolves Open question 3), not an inference made in this repo; the three service functions (`AddWaitingDeliveryCost`/`AddDamageWriteOff`/`AddShortage`) are the only write paths that ever touch their respective discriminator, and each sets it to exactly one constant value, so there is no code path that could leave it ambiguous. |
| Optional remark (spec AC5, AC6) is not implemented anywhere in this repo | Intentional - the remark belongs to `arrival-service`'s `Arrival.shortRemark` per the spec's `## Repos` `arrival-service` paragraph, not to the finance-facing amount owned by `expenses-service`. Confirmed against the spec before this plan was written, not an omission discovered during implementation. |

## Open questions

- None blocking - the AC1 mechanism, route shapes, and repo assignment were all resolved in the specification (`docs/specifications/inbound-shortage-cost.md`, `## Repos`/`## Decision log` sections) before this implementation started.
