# Architecture: Record a damage write-off amount tied to a delivery

## 1. Scope and constraints

- **Problem**: `expenses-service` has no way to record a mandatory write-off money amount tied to a delivery id and an identified operator, distinct from the shipped `waiting-delivery-cost` amount for the same delivery. The existing `POST /expenses/rest/add` accepts any `Expense` with an optional `UserID` and no delivery concept at all; the existing `waiting-cost` path has no discriminator to tell its records apart from a future damage write-off sharing the same `DeliveryID`.
- **Goals**: A new, additional creation path that requires all three of delivery id, an identified (auth-service-validated) operator, and an amount - reject if any is missing/invalid - and marks the resulting record as a damage write-off so it stays distinguishable from a wait-cost record for the same delivery (AC8). A new read-by-id path returning delivery id, user id, and amount.
- **Non-goals**: The warehouse-facing "mark delivery as damaged + optional remark" half (owned by `arrival-service`, out of this repo). No remark/text field here. No list-all endpoint for damage-writeoff records. No live call to `arrival-service` to confirm the delivery id exists - presence of the id is enough, matching the `waiting-delivery-cost` precedent. No change to `POST /expenses/rest/add`, `GET /expenses/rest/all`, or any `waiting-cost*` route/behavior.
- **Constraints**: Go 1.18, Gin, GORM, Postgres - same stack as `waiting-delivery-cost`, confirmed from `go.mod`/`database/config.go`. No new external dependency added - the existing `authclient.HTTPUserValidator` is reused as-is. No new test dependency added - reuses the existing fake-repository/fake-validator pattern.
- **Sources used**: `docs/specifications/damaged-inbound-writeoff.md` (scratch-ai-skills-pilot, read-only reference, already resolves the AC8 mechanism as a new `Expense.IsDamageWriteOff` field and the two new routes); this repo's `model/expense.go`, `service/waiting_delivery_cost.go`, `api/waiting_delivery_cost.go`, `api/api.go`, `authclient/client.go` (all reused/extended, not reimplemented).

## 2. Existing shape to respect

- `api.SetupRoutes` groups everything under `/expenses/rest` and already wires four handlers: `AddExpenseHandler`/`GetExpensesHandler` (unvalidated, untouched) and `AddWaitingDeliveryCostHandler`/`GetWaitingDeliveryCostHandler` (the sibling story - validated, untouched).
- `model.Expense` already carries `DeliveryID int` (added for `waiting-delivery-cost`) and `UserID`/`Amount` from the original struct. This story reuses all three rather than duplicating them - the spec's Cross-service contracts section decided a discriminator field, not a second set of fields.
- `service.waiting_delivery_cost.go` already defines the `UserValidator`/`ExpenseRepository` interfaces, the `ValidationError`/`UpstreamError` types (with `newValidationError`/`newUpstreamError` helpers), and `DefaultExpenseRepository` (GORM-backed). All of these are reused as-is by this story - no redefinition, no duplication.
- `database.MigrateDatabase()` runs `DB.AutoMigrate(&model.Expense{})` on every startup - the new `IsDamageWriteOff` column is picked up automatically, no manual migration step needed, same as `DeliveryID` was.
- `authclient.HTTPUserValidator` (auth-service client) has no changes to make - it is reused exactly as `waiting-delivery-cost` wired it.

## 3. Recommended change

| Component | Responsibility | Notes |
| --- | --- | --- |
| `model.Expense` | Add `IsDamageWriteOff bool` field | Reuses the existing `Expense`/`expenses` table and its `DeliveryID`/`UserID`/`Amount` fields. Zero value (`false`) is what every existing record - both plain `/add` records and `waiting-delivery-cost` records - already has, so this is a backward-compatible additive column. Only `service.AddDamageWriteOff` sets it `true`. |
| `service` (new file `damage_writeoff.go`) | Business rules for the new path | Reuses the existing `UserValidator`/`ExpenseRepository` interfaces and `ValidationError`/`UpstreamError` types from `waiting_delivery_cost.go` - no redefinition. `AddDamageWriteOff` validates delivery id / user id / amount presence, calls the validator, sets `IsDamageWriteOff = true`, then persists via the repository. `GetDamageWriteOff` reads by id via the repository. `AddWaitingDeliveryCost`/`GetWaitingDeliveryCost` are untouched. |
| `api` (new file `damage_writeoff.go`) | New HTTP surface | `AddDamageWriteOffHandler` (`POST /expenses/rest/damage-writeoff`) and `GetDamageWriteOffHandler` (`GET /expenses/rest/damage-writeoff/:id`), mirroring the sibling's structure exactly: package-level `damageWriteOffValidator`/`damageWriteOffRepo` vars wired to the real `authclient.HTTPUserValidator`/`service.DefaultExpenseRepository` in production, swappable in tests. Same error-to-status mapping (400 validation, 502 upstream, 500 other, 404 not-found, 400 non-numeric id). |
| `api.SetupRoutes` | Route registration | Adds the two new routes under the existing `/expenses/rest` group; all four existing routes are unchanged (same path, same handler, same order). |
| `database` | Migration | No code change - `AutoMigrate(&model.Expense{})` already covers the new `IsDamageWriteOff` column. |

- Happy path (create): handler binds JSON → `service.AddDamageWriteOff` checks delivery id / user id / amount are non-zero → calls `UserValidator.UserExists(userID)` → on `true`, sets `IsDamageWriteOff = true` and persists via `ExpenseRepository.Create` → 200 with `{id, deliveryId, userId, amount}` (the generated `id` included, same pointer-parameter reasoning as `AddWaitingDeliveryCost`, so the caller can retrieve the record afterwards).
- Validation failure (missing field, or auth-service reports the user id doesn't exist): `service.ValidationError` → handler returns 400.
- Auth-service unreachable / unexpected response: `service.UpstreamError` → handler returns 502.
- Persistence failure (DB error after all validation passed): plain `error` → handler returns 500.
- Read: handler parses `:id` → `service.GetDamageWriteOff` → 404 if not found (`gorm.ErrRecordNotFound`), 500 for any other DB error, 200 with `{id, deliveryId, userId, amount}` otherwise. `IsDamageWriteOff` is not exposed in the response body - it is a storage-level discriminator, not part of the finance-facing read shape (mirrors `waiting-delivery-cost`'s response shape, which also doesn't expose `DeliveryID`'s sibling-path origin).

## 4. Key decisions and risks

| Decision | Options considered | Choice | Rationale |
| --- | --- | --- | --- |
| AC8 mechanism | New table/model vs. a discriminator field on the existing `Expense`/table | New field (`IsDamageWriteOff bool`), reusing `DeliveryID`/`UserID`/`Amount` | Already resolved by the specification's Cross-service contracts section before this implementation started. A damage-writeoff record and a wait-cost record are structurally identical (delivery id + validated user + amount); a boolean flag is sufficient to keep them business-distinguishable without duplicating fields or introducing a second table. |
| Interface/error-type reuse | Redefine `UserValidator`/`ExpenseRepository`/`ValidationError`/`UpstreamError` in the new file vs. reuse the sibling's | Reuse as-is from `service/waiting_delivery_cost.go` | Both paths share identical validation shape and persistence needs; redefining would either collide (same package, same names) or fork behavior for no reason. Reusing keeps `AddDamageWriteOff` and `AddWaitingDeliveryCost` provably consistent in how they treat validation and errors. |
| Response body shape | Expose `isDamageWriteOff` in `DamageWriteOffResponse` vs. omit it | Omit | The route itself (`/damage-writeoff` vs `/waiting-cost`) and the response type (`DamageWriteOffResponse` vs `WaitingDeliveryCostResponse`) already tell the caller which kind of record it's looking at; exposing the internal flag adds no information the caller doesn't already have from which endpoint it called. |
| Test doubles for persistence | GORM against sqlite in-memory vs. reusing the sibling's fake `ExpenseRepository`/`UserValidator` | Reuse `fakeExpenseRepository`/`fakeUserValidator` (service package) and `waitingCostFakeRepo`/`fakeValidator` (api package) | Same rationale as `waiting-delivery-cost`: no new dependency, and reusing the exact same fakes lets the new AC8 cross-story test create both record kinds in one shared fake repository instance. |

## 5. Implementation handoff

- Touch areas: `model/expense.go` (add field), `service/damage_writeoff.go` (new), `api/damage_writeoff.go` (new), `api/api.go` (register two routes). No change to `authclient/client.go`, `service/waiting_delivery_cost.go`, `api/waiting_delivery_cost.go`, `api/expense.go`, `service/expense.go`, or `database/config.go`.
- Tests: `service/damage_writeoff_test.go`, `api/damage_writeoff_test.go` - table-driven where the shape fits, reusing the sibling's fakes and `setupRouterForTest`/`performRequest` helpers. Includes a dedicated AC8 test at both the service and handler level: create a wait-cost record and a damage-writeoff record for the same `DeliveryID` in the same fake repository, then fetch both back and assert `IsDamageWriteOff` correctly distinguishes them.
- Config surface: none added - reuses `AUTH_SERVICE_BASE_URL` (already present for `waiting-delivery-cost`).
- Branch: `feature/damaged-inbound-writeoff` (already created, cut from `main`).
