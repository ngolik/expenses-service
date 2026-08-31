# Expense: record which user submitted the expense

## Goal

An operator creating or viewing an expense can record which existing system
user submitted it, so finance and audit can see who originated the claim
without typing a free-text name.

## In scope

- On create and on get/view of an expense, the operator can set and see the
  submitting user (identity from the existing user catalog).
- Invalid or unknown user identity is rejected with a clear client error.
- Existing expense fields and list/get behaviour stay available.

## Out of scope / non-goals

- New authentication or registration flows
- Changing how users are stored or how login works
- UI beyond the HTTP API used by operators
- Eureka / service-registry configuration
- Historical backfill of old expenses
- OpenAPI generator or major framework upgrades

## Constraints

- Stay on the **existing expenses-service stack** (Go / Gin / GORM); do **not**
  silently change major versions. Java/Spring applies to auth-service only
  (consumed as-is, no change in this story).
- `layout:` n/a for this repo (not `spring-layers`)
- Thin architecture (omit `complexity: high`)
- Reuse existing user identity; do not duplicate a second user table

## Unchanged contracts

- Existing expense create/list keep working for callers that omit `UserID`
- Unrelated services and routes stay as they are

## Acceptance criteria

- [ ] Create expense can include optional submitting user; that user is an
      existing system user when the field is sent
- [ ] Get-by-id returns the expense and echoes the stored `UserID` (the id sent,
      not parsed from auth `UserDTO`)
- [ ] Unknown user identity is rejected (4xx); omitted `UserID` is allowed
- [ ] Tests cover happy path + omitted field + unknown-user validation and pass
      via project build
- [ ] Architecture + impl-plan docs exist under `docs/` in this repo

## Open questions

- Submitting user on create: **optional** (resolved in coordinator memo)
- Echo id: store/return the id **sent** by the caller, not from `UserDTO`

## Sources

- Practice hub spec: `scratch-ai-skills-pilot/docs/specifications/expense-submitted-by-user.md`
- Coordinator memo in that file (`## Repos`, contracts, Order)

## Suggested slug

`expense-submitted-by-user`

## Tracker

Not linked. Local multi-repo coordinator pilot.

## Repos

See practice-hub specification (same slug). This worktree implements
**expenses-service only**; auth-service is consumed as-is.
