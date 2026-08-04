# Costs & Infra: backend architecture

How cloud billing data gets from a customer's AWS account into the Costs &
Infra tab, how often it refreshes, and what keeps the database from growing
without bound. Scoped to `internal/billing/*` in this repo.

## Data flow

```
Settings > Cloud Connections           scheduled worker (every 6h)
  "Sync now" (async, returns          internal/billing/sync.Worker.Run
   immediately, UI polls status)              |
        |                                     |
        +------------------+------------------+
                           v
              billing.RunSync (5-min timeout)
                           |
        decrypt stored credentials (AES-256-GCM)
                           |
              ProviderAdapter.Sync(ctx, creds)
                           |
        +------------------+------------------+
        v                                     v
Resource Groups Tagging API          Cost Explorer GetCostAndUsage
(fan out to all 29 AWS regions,      (account-wide, last 30 days,
 8 concurrent, region opt-out         grouped by AWS billed service —
 failures skipped, not fatal)         no per-resource dimension exists)
        |                                     |
        v                                     v
  exact resource inventory      allocateCosts(): split each day's
  + real tags, per region       per-service total evenly across
                                 resources billed under that service
        +------------------+------------------+
                           v
              UPSERT cost_resources
              UPSERT cost_usage_daily
                           |
                update cloud_connections.status
              (syncing -> connected | error)
```

A resource is never assigned to a service at sync time — `cost_resources`
rows are org-scoped only. `service_cost_tag_rules` (configured per service
in the UI) is matched against a resource's real tags at **read** time
(`ListResourcesForService`, `ListTrendForService`), so changing a service's
tag rules re-slices existing history instantly with no re-sync required.

## Sync cadence

| Trigger | Interval | Notes |
|---|---|---|
| Scheduled worker | every 6h, plus once on `api` container boot | one goroutine in the API binary, no separate service |
| Manual "Sync now" | on demand | same `RunSync` path, fired in a detached goroutine so the HTTP request returns immediately |
| Per-sync timeout | 5 min hard cap | applies to both paths — one hung connection can't block the batch or hang forever |
| Cost Explorer window | last 30 days, every sync | AWS billing data itself lags 24–48h, so faster polling wouldn't surface anything newer |

Billing data is inherently slow-moving, so a ticker in the existing API
process is enough — there's no cron system or separate worker binary.

## Ensuring data stays synced

- `cloud_connections.status` is the source of truth for freshness:
  `pending` (never verified) → `syncing` (in flight) → `connected` (last
  sync succeeded) or `error` (last sync failed, with `status_message` set
  to the actual error).
- Every sync attempt — scheduled or manual — writes a `cost_sync_runs` row
  (`started_at`, `finished_at`, `status`, `resource_count`,
  `error_message`), giving an audit trail independent of the connection's
  current status. Not yet surfaced in the UI — worth adding.
- Region discovery failures are **not** treated as sync failures individually
  (most accounts aren't opted into all 29 commercial regions — that's
  normal). Only if *every* region fails does the sync report an error, on
  the theory that's a real credential/permission problem rather than
  unused regions.
- Each `cost_resources` row carries `last_synced_at`, so a resource that
  silently stopped syncing (without the connection itself erroring) is
  identifiable — though nothing currently *acts* on a stale timestamp (see
  Known gaps below).

## Data model & growth characteristics

All writes are `INSERT ... ON CONFLICT ... DO UPDATE` — every table below
is upserted, not appended. Re-syncing the same resource on the same day
overwrites in place; it never creates a duplicate row.

| Table | Grain | Growth driver | Bounded by sync frequency? |
|---|---|---|---|
| `cloud_connections` | 1 row / connected account | connections created | yes — human-driven, tiny |
| `service_cost_tag_rules` | 1 row / tag rule | rules configured in UI | yes — human-driven, tiny |
| `cost_resources` | 1 row / resource (unique on `cloud_connection_id, external_resource_id`) | **actual resource count**, not sync count | **yes** — resyncing hourly vs. daily doesn't add rows |
| `cost_usage_daily` | 1 row / resource / calendar day (unique on `resource_id, usage_date`) | resource count × **new calendar days elapsed** | **no** — grows every day, forever, regardless of sync frequency |
| `cost_sync_runs` | 1 row / sync attempt | sync frequency × connection count | **no** — grows every sync, forever |

The two unbounded tables are `cost_usage_daily` and `cost_sync_runs` — sync
frequency doesn't affect the first (upserts absorb it) but does directly
drive the second.

### Example projection

A mid-size org: 5 connected AWS accounts, ~200 tagged resources each
(1,000 resources total) — a realistic ceiling for the supported resource
types (EC2, RDS, DynamoDB, S3, SQS, ELB, Lambda, ElastiCache, EKS).

| Table | After 1 year | After 3 years |
|---|---|---|
| `cost_resources` | 1,000 rows (flat — bounded by actual inventory) | 1,000 rows |
| `cost_usage_daily` | 1,000 × 365 ≈ **365,000 rows** | ≈ **1.1M rows** |
| `cost_sync_runs` | 5 connections × 4 syncs/day × 365 ≈ **7,300 rows** | ≈ **21,900 rows** |

None of this is alarming for Postgres on its own (a few million narrow,
indexed rows is routine), but it is **unbounded** — there is currently no
retention policy, so it grows forever rather than plateauing.

## Known gaps (not yet implemented)

1. **No retention/pruning job.** `cost_usage_daily` and `cost_sync_runs`
   grow indefinitely. Recommended: a periodic job (same ticker-goroutine
   shape as the sync worker) deleting `cost_usage_daily` rows older than
   ~13–24 months (AWS Cost Explorer itself only retains ~14 months) and
   `cost_sync_runs` older than ~90 days or beyond the last N per
   connection.
2. **No stale-resource cleanup.** If a resource is deleted in AWS (or
   detagged), its `cost_resources` row is never removed — it just stops
   getting a fresh `last_synced_at`. Recommended: on each sync, delete (or
   flag) resources under a connection whose `external_resource_id` wasn't
   seen in that pass.
3. **`cost_sync_runs` history isn't surfaced in the UI** — the data exists
   (status, error, resource count per attempt) but there's no "last 10
   syncs" view yet.

## What "cost" actually means here

Cost Explorer's public API has no per-resource cost dimension without an
opt-in Cost & Usage Report (CUR) export to S3 — that's a materially bigger
integration (Athena/Glue, or parsing CUR files directly). Absent that, a
day's total for an AWS-billed service (e.g. "Amazon Elastic Compute Cloud -
Compute") is split **evenly** across every discovered resource billed
under that service that day. This is exact at the service-total level and
an approximation at the resource level — a large EC2 instance and a small
one currently show the same daily cost if they're the only two running.
Good enough for "which service categories are expensive and trending
which way"; not a substitute for CUR-based cost allocation if per-resource
precision matters.

## Security

Credentials are AES-256-GCM encrypted at rest (`internal/crypto`, same
cipher used for Figma OAuth tokens) and decrypted only in-process for the
duration of a sync or test call. AWS specifically never requires a stored
long-lived access key — the connection form takes a role ARN + external ID,
and the sync worker calls `sts:AssumeRole` against base credentials
configured on the `api` service itself.
