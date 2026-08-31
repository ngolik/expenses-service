# Delivery log (ROI + per-feature cost)

Lightweight measurement **without Dash0**. One row per feature after G5 / merge.
**Does not block delivery.**

Per-feature detail: `docs/ai-context/features/<slug>/cost-summary.json`

## Cost basis (important)

| Cost basis | Meaning |
| --- | --- |
| **measured** | `cost_usd` reported in Claude session records |
| **estimated** | **Projected** from token counts + list prices — **not vendor billing** |
| **mixed** | Some measured, some projected |
| **none** | No session tokens found for the feature window |

Never present **estimated** as invoiced cost.

## Log

| Date | Slug | Wall time | AI cost (USD) | Cost basis | Uncached tok | Cache tok | Out tok | Messages | Rework (Y/N) | Escaped (n) | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-08-25 | expense-submitted-by-user |  | $2.2067 (estimated) | estimated | 92 | 4215155 | 38449 | 46 |  |  |  |
