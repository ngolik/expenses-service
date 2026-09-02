# Implementation Plan: Record a damage write-off amount tied to a delivery

## Summary

- **Goal**: Add a new creation path that requires delivery id + auth-service-validated user id + amount, marks the record as a damage write-off (distinct from a `waiting-delivery-cost` record for the same delivery, AC8), and a read-by-id path for the created record - without touching the existing `/expenses/rest/add`, `/expenses/rest/all`, or `/expenses/rest/waiting-cost*` behavior.
- **In scope**: `model.Expense.IsDamageWriteOff` field; `service.AddDamageWriteOff` / `service.GetDamageWriteOff` (reusing the sibling's `UserValidator`/`ExpenseRepository` interfaces and `ValidationError`/`UpstreamError` types); `POST /expenses/rest/damage-writeoff` and `GET /expenses/rest/damage-writeoff/:id` handlers + routes; unit tests for all of the above, including a cross-story AC8 check.
- **Out of scope**: `arrival-service` changes, remark/text field, list-all endpoint, live existence-check against `arrival-service`, any change to `AddExpenseHandler`/`GetExpensesHandler`/`AddExpense`/`GetAllExpenses`/`AddWaitingDeliveryCostHandler`/`GetWaitingDeliveryCostHandler`/`AddWaitingDeliveryCost`/`GetWaitingDeliveryCost`.
- **Architecture input**: `docs/architecture/damaged-inbound-writeoff.md`

## Touch map

| Area | Why |
| --- | --- |
| `model/expense.go` | Add `IsDamageWriteOff bool` field to `Expense` |
| `service/damage_writeoff.go` (new) | `AddDamageWriteOff`, `GetDamageWriteOff` - reuses `UserValidator`/`ExpenseRepository`/`ValidationError`/`UpstreamError` from `waiting_delivery_cost.go`, no redefinition |
| `service/damage_writeoff_test.go` (new) | Table-driven tests against the existing fake repository + fake validator: all-fields-present success (including `IsDamageWriteOff == true` on the stored record), each missing-field rejection, unknown-user rejection, validator-error → upstream error, repository-error passthrough, get-found, get-not-found, and a dedicated AC8 test creating both record kinds for the same `DeliveryID` |
| `api/damage_writeoff.go` (new) | `AddDamageWriteOffHandler`, `GetDamageWriteOffHandler`, package-level default validator/repository vars |
| `api/damage_writeoff_test.go` (new) | `httptest` + `gin.TestMode` tests against the handlers with fakes substituted for the package vars: 200 create, 400 per validation case, 502 upstream, 500 db failure, 400 malformed body, 200 get, 404 get, 400 non-numeric id, 500 other repo error, and a handler-level AC8 check |
| `api/api.go` | Register the two new routes in the existing `/expenses/rest` group |

No changes to `authclient/client.go`, `service/waiting_delivery_cost.go`, `service/waiting_delivery_cost_test.go`, `api/waiting_delivery_cost.go`, `api/waiting_delivery_cost_test.go`, `api/expense.go`, `service/expense.go`, `database/config.go`, `main.go`.

## Steps

### Step 1 - Model: add `IsDamageWriteOff`

- **Outcome**: `Expense` carries a discriminator that stays `false` for every existing record (plain `/add` and `waiting-delivery-cost`) and is only ever `true` for a damage-writeoff record; `AutoMigrate` adds the column on next startup.
- **Approach**: Add `IsDamageWriteOff bool` to the `Expense` struct in `model/expense.go`, with a comment explaining the discriminator purpose (mirrors `DeliveryID`'s comment style).
- **Tests**: None standalone - exercised via Step 2/3 tests.
- **Done when**: Struct compiles; no other file needs to change for this step alone.

### Step 2 - `service`: validation + persistence for the new path

- **Outcome**: A function that enforces "delivery id, validated user id, amount all present" and marks the record as a damage write-off before persisting, plus a function to read one record back.
- **Approach**: New file `service/damage_writeoff.go`. Reuses `UserValidator`, `ExpenseRepository`, `ValidationError`, `UpstreamError`, `newValidationError`, `newUpstreamError` from `waiting_delivery_cost.go` (same package, no redefinition). `AddDamageWriteOff(expense *model.Expense, validator UserValidator, repo ExpenseRepository) error`: reject with `ValidationError` if `DeliveryID == 0`, `UserID == 0`, or `Amount == 0`; call `validator.UserExists(expense.UserID)` - a transport/unexpected-status error becomes `UpstreamError`, a clean "not found" becomes `ValidationError`; otherwise set `expense.IsDamageWriteOff = true` and call `repo.Create(expense)`, returning its error as-is. `GetDamageWriteOff(id uint, repo ExpenseRepository) (*model.Expense, error)`: `repo.FindByID(id)`.
- **Tests**: `service/damage_writeoff_test.go`, table-driven, reusing the existing `fakeExpenseRepository`/`fakeUserValidator` from `waiting_delivery_cost_test.go` (same package). Cases: happy path (record created, `IsDamageWriteOff == true`, generated id visible on the caller's pointer); missing delivery id; missing user id; missing amount; validator reports user does not exist; validator returns a transport error (→ `UpstreamError`, nothing persisted); repository `Create` error surfaces unchanged; `GetDamageWriteOff` found; `GetDamageWriteOff` not found. Plus `TestAC8_WaitCostAndDamageWriteOffAreDistinguishable`: creates a wait-cost record and a damage-writeoff record for the same `DeliveryID` in one shared fake repository, fetches both back, asserts `IsDamageWriteOff` is `false` on the wait-cost one and `true` on the damage one.
- **Done when**: All cases pass without touching `database.DB` or any network call.

### Step 3 - `api`: handlers + routes

- **Outcome**: `POST /expenses/rest/damage-writeoff` and `GET /expenses/rest/damage-writeoff/:id` are reachable and return the right status codes.
- **Approach**: New file `api/damage_writeoff.go`, mirroring `api/waiting_delivery_cost.go` structure exactly. Package-level vars `damageWriteOffValidator service.UserValidator = authclient.NewHTTPUserValidator()` and `damageWriteOffRepo service.ExpenseRepository = service.DefaultExpenseRepository`. `AddDamageWriteOffHandler`: `c.BindJSON(&model.Expense{})` (400 on bind failure), then `service.AddDamageWriteOff(&expense, damageWriteOffValidator, damageWriteOffRepo)`; `errors.As` against `*service.ValidationError` → 400, `*service.UpstreamError` → 502, anything else → 500; success → 200 + `DamageWriteOffResponse{id, deliveryId, userId, amount}`. `GetDamageWriteOffHandler`: parse `:id` (400 on non-numeric), `service.GetDamageWriteOff`; `gorm.ErrRecordNotFound` → 404, other error → 500, success → 200 + the same response shape. Register both routes in `api.SetupRoutes` alongside the existing four, same group, no reordering of existing lines.
- **Tests**: `api/damage_writeoff_test.go`, reusing `setupRouterForTest`/`performRequest`/`fakeValidator`/`waitingCostFakeRepo` from `waiting_delivery_cost_test.go` (same package). Cases: 200 create with a non-zero `id` in the response body; 400 for each missing field; 400 for unknown user; 502 when the validator errors; 500 on db create failure; 400 for malformed JSON body; 200 get with correct body shape; 404 get for missing id; 400 get for non-numeric id; 500 get for other repository error. Plus `TestAC8_HandlerLevel_WaitCostAndDamageWriteOffAreDistinguishable`: creates a wait-cost record and a damage-writeoff record for the same `DeliveryID` through the real handlers/routes against a shared fake repo, checks each response body's fields independently and that the discriminator is set correctly for each.
- **Done when**: All cases pass; `AddExpenseHandler`/`GetExpensesHandler`/`AddWaitingDeliveryCostHandler`/`GetWaitingDeliveryCostHandler` and their existing routes are untouched (verified by re-reading `api/api.go` diff - only additive lines).

### Step 4 - Full-repo verification

- **Outcome**: Confirm nothing else broke.
- **Approach**: From ROOT, run `go build ./...` then `go test ./...`.
- **Tests**: All of the above, run together.
- **Done when**: Both commands exit 0.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Reusing the `Expense` table for damage-writeoff records means `GET /expenses/rest/all` will now also return damage-writeoff rows (with `IsDamageWriteOff` populated) alongside plain expenses and wait-cost rows | Acceptable, matching the `waiting-delivery-cost` precedent - no list-all endpoint is required for damage-writeoff records (AC9 explicitly puts a damaged-deliveries list out of scope), and the spec doesn't require excluding them from the existing all-expenses list. Not treated as a behavior change to `/all`'s contract. |
| `IsDamageWriteOff` not exposed in `DamageWriteOffResponse`/`WaitingDeliveryCostResponse` | Intentional (see architecture doc §4) - the route/response-type pair already discriminates for the caller; the flag is a storage-level implementation detail for AC8, not part of either read contract. If a future consumer needs the raw flag (e.g. a combined view across both record types), that is a new, separate read shape, not a change to either existing response. |
| This story's write-off amount and the sibling's wait-cost amount live in the same table/columns, distinguished only by a boolean | This is the approved AC8 mechanism per the specification's Cross-service contracts section, not an inference made in this repo; both service functions (`AddWaitingDeliveryCost`/`AddDamageWriteOff`) are the only two write paths that ever touch `IsDamageWriteOff`, and each sets it to exactly one constant value, so there is no code path that could leave it ambiguous. |

## Open questions

- None blocking - the AC8 mechanism, route shapes, and repo assignment were all resolved in the specification (`docs/specifications/damaged-inbound-writeoff.md`, Cross-service contracts / Open questions sections) before this implementation started.
