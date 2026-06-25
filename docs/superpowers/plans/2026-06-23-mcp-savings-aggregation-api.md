# MCP Cost-Savings Aggregation API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add blended (all-model) support to the existing MCP savings summary endpoint, plus four new aggregation endpoints (daily time series, by-tool, by-model, by-user) so the uigraph-ui "Insights" dashboard can show trends and breakdowns, not just a single period total.

**Architecture:** Pure query-layer additions on the existing `mcp_usage_events`/`llm_models` tables — no new migration. Each new endpoint follows the exact shape of the existing `GetSavingsSummary`/`Summary` handler: a `internal/store/postgres/mcp_usage.go` SQL function, a `internal/api/mcpusage` HTTP handler, a route registered in `Register()`. All new SQL uses `LEFT JOIN llm_models m ON m.model_id = e.model_id` (fixing the existing `GetSavingsSummary`'s `CROSS JOIN ... WHERE m.model_id = $2` bug, which only ever priced events against one externally-supplied model regardless of which model they actually used) so each event is priced against its own model, and an optional `model_id` filter (`AND ($N = '' OR e.model_id = $N)`) supports both "blended across all models" (param omitted) and "filtered to one model" (param provided, preserving today's exact behavior).

**Tech Stack:** Go, stdlib `net/http.ServeMux` (1.22+ pattern routing, `r.PathValue`), `database/sql` + `lib/pq`, Postgres. No web framework, no ORM, no ginkgo/testify — plain `testing` package throughout.

## Global Constraints

- Module path: `github.com/uigraph/app`
- No new database migration — all five endpoints query existing columns on `mcp_usage_events` and `llm_models`
- Period query param is always one of `1d|7d|30d|1y`, default `7d`, parsed via the new shared `parsePeriod` helper (Task 1)
- `model_id` query param is optional everywhere it appears: omitted = blended across all models (each event priced at its own model's rate via the `LEFT JOIN`); provided = filtered to that one model
- JSON field names are camelCase via Go struct tags, matching the existing `UsageEvent`/`SavingsSummary` convention
- All new handler routes use `requireScope("services:read", "GET", "<pattern>", h.<Method>)` in `internal/api/mcpusage/handler.go`'s `Register`, matching the existing `Summary`/`List` registrations
- Tests are end-to-end HTTP tests added to `tests/mcp_savings_test.go` (new file, `package tests`), reusing the shared `TestMain`-built `srv`, `adminToken`, `orgID`, and the `do`/`mustDo`/`str`/`list`/`M` helpers already defined in `tests/api_test.go` — no testify, no new test infrastructure
- Running tests requires a reachable Postgres at `TEST_POSTGRES_URL` (default `postgres://uigraph:devpassword@localhost:5432/uigraph?sslmode=disable`); if unreachable, `TestMain` prints `SKIP: postgres unavailable` and calls `os.Exit(0)` — the whole suite reports as passed with zero tests run, which is **not** the same as your new tests passing. Confirm your specific test function actually ran (look for its name in `go test -v` output) before treating a green run as proof.
- Run from the repo root: `/Users/kranthi/workspace/go/uigraph/backend/uigraph-oss/uigraph-api`

---

### Task 1: Blended summary support + shared period parsing

**Files:**
- Modify: `internal/store/postgres/mcp_usage.go` (the `GetSavingsSummary` function)
- Create: `internal/api/mcpusage/period.go`
- Modify: `internal/api/mcpusage/summary.go`
- Create: `tests/mcp_savings_test.go`

**Interfaces:**
- Produces: `parsePeriod(raw string) (period string, since time.Time)` — used by every handler added in Tasks 2-5
- Produces (unchanged signature, changed semantics): `(d *DB) GetSavingsSummary(ctx context.Context, orgID, modelID string, since time.Time) (*mcpusage.SavingsSummary, error)` — `modelID == ""` now means blended

- [ ] **Step 1: Write the failing tests**

Create `tests/mcp_savings_test.go`:

```go
// Package tests — see api_test.go for shared TestMain setup, adminToken, orgID.
package tests

import "testing"

func TestMCPSavings_Summary_Blended(t *testing.T) {
	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y", adminToken, nil)
	beforeCalls := int(before["totalCalls"].(float64))
	beforeSaved := int(before["totalTokensSaved"].(float64))

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "get_service_context", "resourceIds": []string{"svc-1"},
		"modelId": "claude-sonnet-4-6", "tokensServed": 100, "tokensRawEquivalent": 600,
		"tokensSaved": 500, "responseSizeBytes": 2048,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y", adminToken, nil)
	afterCalls := int(after["totalCalls"].(float64))
	afterSaved := int(after["totalTokensSaved"].(float64))

	if afterCalls != beforeCalls+1 {
		t.Fatalf("want totalCalls to increase by 1, before=%d after=%d", beforeCalls, afterCalls)
	}
	if afterSaved != beforeSaved+500 {
		t.Fatalf("want totalTokensSaved to increase by 500, before=%d after=%d", beforeSaved, afterSaved)
	}
	if after["modelId"] != "" {
		t.Fatalf("want blended summary to report empty modelId, got %v", after["modelId"])
	}
}

func TestMCPSavings_Summary_FilteredByModel(t *testing.T) {
	const other = "claude-haiku-4-5"
	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y&model_id="+other, adminToken, nil)
	beforeCalls := int(before["totalCalls"].(float64))

	// An event for a DIFFERENT model must not affect the filtered summary.
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "get_api_spec", "resourceIds": []string{"svc-1"},
		"modelId": "gpt-4o", "tokensServed": 10, "tokensRawEquivalent": 40,
		"tokensSaved": 30, "responseSizeBytes": 256,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y&model_id="+other, adminToken, nil)
	afterCalls := int(after["totalCalls"].(float64))

	if afterCalls != beforeCalls {
		t.Fatalf("want totalCalls unaffected by a different model's event, before=%d after=%d", beforeCalls, afterCalls)
	}
	if after["modelId"] != other {
		t.Fatalf("want modelId %q echoed back, got %v", other, after["modelId"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (or rather, do not yet reflect blended behavior)**

Run: `go test ./tests/... -run TestMCPSavings_Summary -v`

Expected: `TestMCPSavings_Summary_Blended` FAILs on the `after["modelId"] != ""` check — today's handler defaults empty `model_id` to `"claude-sonnet-4-6"`, so `after["modelId"]` is currently `"claude-sonnet-4-6"`, not `""`. (`TestMCPSavings_Summary_FilteredByModel` likely passes already since it always supplies an explicit `model_id` — that's expected; the meaningful new-behavior assertion is in `_Blended`.)

- [ ] **Step 3: Fix the join + make model_id optional in the store layer**

In `internal/store/postgres/mcp_usage.go`, replace the `GetSavingsSummary` function body:

```go
func (d *DB) GetSavingsSummary(ctx context.Context, orgID, modelID string, since time.Time) (*mcpusage.SavingsSummary, error) {
	const q = `
		SELECT
		    COUNT(*)                                                                                AS total_calls,
		    COALESCE(SUM(e.tokens_served), 0)                                                        AS total_tokens_served,
		    COALESCE(SUM(e.tokens_saved), 0)                                                         AS total_tokens_saved,
		    COALESCE(SUM(e.tokens_served::NUMERIC / 1000000 * m.input_cost_per_million), 0)          AS cost_served_usd,
		    COALESCE(SUM(e.tokens_raw_equivalent::NUMERIC / 1000000 * m.input_cost_per_million), 0)  AS cost_raw_usd,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0)           AS cost_saved_usd,
		    COUNT(DISTINCT e.user_id)                                                                 AS unique_users_count
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)`
	var s mcpusage.SavingsSummary
	err := d.db.QueryRowContext(ctx, q, orgID, since, modelID).Scan(
		&s.TotalCalls, &s.TotalTokensServed, &s.TotalTokensSaved,
		&s.CostServedUSD, &s.CostRawUSD, &s.CostSavedUSD,
		&s.UniqueUsersCount,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsSummary: %w", err)
	}
	s.OrgID = orgID
	s.ModelID = modelID
	return &s, nil
}
```

(Note the placeholder order changed from the original `$1=orgID, $2=modelID, $3=since` to `$1=orgID, $2=since, $3=modelID` — this is a self-contained rewrite, so there's no caller-visible signature change to worry about.)

- [ ] **Step 4: Add the shared period-parsing helper**

Create `internal/api/mcpusage/period.go`:

```go
package mcpusage

import "time"

// parsePeriod converts a period query param ("1d", "7d", "30d", "1y") into its
// canonical string and the UTC timestamp it resolves to. Unrecognized or empty
// values default to "7d".
func parsePeriod(raw string) (period string, since time.Time) {
	now := time.Now().UTC()
	switch raw {
	case "1d":
		return "1d", now.Add(-24 * time.Hour)
	case "30d":
		return "30d", now.Add(-30 * 24 * time.Hour)
	case "1y":
		return "1y", now.Add(-365 * 24 * time.Hour)
	default:
		return "7d", now.Add(-7 * 24 * time.Hour)
	}
}
```

- [ ] **Step 5: Update the Summary handler to stop defaulting model_id and use the shared helper**

Replace the full contents of `internal/api/mcpusage/summary.go`:

```go
package mcpusage

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
)

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	period, since := parsePeriod(q.Get("period"))

	summary, err := h.store.GetSavingsSummary(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	summary.Period = period
	httputil.JSON(w, http.StatusOK, summary)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./tests/... -run TestMCPSavings_Summary -v`

Expected: `PASS` for both `TestMCPSavings_Summary_Blended` and `TestMCPSavings_Summary_FilteredByModel`.

- [ ] **Step 7: Run the full test suite to confirm no regressions**

Run: `go test ./... `

Expected: all packages `ok`, no failures (the `tests` package requires Postgres as described in Global Constraints).

- [ ] **Step 8: Commit**

```bash
git add internal/store/postgres/mcp_usage.go internal/api/mcpusage/period.go internal/api/mcpusage/summary.go tests/mcp_savings_test.go
git commit -m "feat: support blended (all-model) MCP savings summary"
```

---

### Task 2: Daily time-series endpoint

**Files:**
- Modify: `internal/mcpusage/mcpusage.go` (add `DailySavings` type + `Store` interface method)
- Modify: `internal/store/postgres/mcp_usage.go` (add `GetSavingsTimeseries`)
- Modify: `internal/api/mcpusage/handler.go` (extend `store` interface + register route)
- Create: `internal/api/mcpusage/breakdowns.go` (add `Timeseries` handler)
- Modify: `tests/mcp_savings_test.go` (add test, introduce `fmt`/`time` imports)

**Interfaces:**
- Consumes: `parsePeriod` from Task 1
- Produces: `mcpusage.DailySavings` struct (`Date time.Time`, `TotalCalls int`, `TotalTokensServed int`, `TotalTokensSaved int`, `CostServedUSD float64`, `CostRawUSD float64`, `CostSavedUSD float64`), and `(d *DB) GetSavingsTimeseries(ctx, orgID, modelID string, since time.Time) ([]mcpusage.DailySavings, error)` — both consumed by the uigraph-graphql plan's REST client

- [ ] **Step 1: Write the failing test**

Append to `tests/mcp_savings_test.go` (add `"fmt"` and `"time"` to the import block):

```go
package tests

import (
	"fmt"
	"testing"
	"time"
)
```

```go
func TestMCPSavings_Timeseries(t *testing.T) {
	tool := fmt.Sprintf("test-timeseries-tool-%d", time.Now().UnixNano())

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": tool, "resourceIds": []string{"svc-1"},
		"modelId": "claude-sonnet-4-6", "tokensServed": 100, "tokensRawEquivalent": 600,
		"tokensSaved": 500, "responseSizeBytes": 2048,
	})

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/timeseries?period=1y", adminToken, nil)
	days := list(body, "timeseries")
	if len(days) == 0 {
		t.Fatal("expected at least one day bucket")
	}

	today := time.Now().UTC().Format("2006-01-02")
	var totalSavedToday int
	for _, d := range days {
		date, _ := d["date"].(string)
		if len(date) >= 10 && date[:10] == today {
			totalSavedToday = int(d["totalTokensSaved"].(float64))
		}
	}
	if totalSavedToday < 500 {
		t.Fatalf("want today's bucket to include the 500 tokens just saved, got %d", totalSavedToday)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/... -run TestMCPSavings_Timeseries -v`

Expected: FAIL — `GET .../mcp/savings/timeseries` 404s (route doesn't exist), so `mustDo` calls `t.Fatal` on the non-2xx status.

- [ ] **Step 3: Add the `DailySavings` type and `Store` interface method**

In `internal/mcpusage/mcpusage.go`, add below the existing `SavingsSummary` struct:

```go
// DailySavings is one day's aggregated usage/cost-savings totals.
type DailySavings struct {
	Date              time.Time `json:"date"`
	TotalCalls        int       `json:"totalCalls"`
	TotalTokensServed int       `json:"totalTokensServed"`
	TotalTokensSaved  int       `json:"totalTokensSaved"`
	CostServedUSD     float64   `json:"costServedUsd"`
	CostRawUSD        float64   `json:"costRawUsd"`
	CostSavedUSD      float64   `json:"costSavedUsd"`
}
```

And add to the `Store` interface:

```go
GetSavingsTimeseries(ctx context.Context, orgID, modelID string, since time.Time) ([]DailySavings, error)
```

- [ ] **Step 4: Add the Postgres aggregation function**

In `internal/store/postgres/mcp_usage.go`, add:

```go
func (d *DB) GetSavingsTimeseries(ctx context.Context, orgID, modelID string, since time.Time) ([]mcpusage.DailySavings, error) {
	const q = `
		SELECT
		    date_trunc('day', e.created_at)                                                          AS day,
		    COUNT(*)                                                                                  AS total_calls,
		    COALESCE(SUM(e.tokens_served), 0)                                                         AS total_tokens_served,
		    COALESCE(SUM(e.tokens_saved), 0)                                                          AS total_tokens_saved,
		    COALESCE(SUM(e.tokens_served::NUMERIC / 1000000 * m.input_cost_per_million), 0)           AS cost_served_usd,
		    COALESCE(SUM(e.tokens_raw_equivalent::NUMERIC / 1000000 * m.input_cost_per_million), 0)   AS cost_raw_usd,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0)            AS cost_saved_usd
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)
		GROUP BY day
		ORDER BY day ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, since, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsTimeseries: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.DailySavings
	for rows.Next() {
		var row mcpusage.DailySavings
		if scanErr := rows.Scan(
			&row.Date, &row.TotalCalls, &row.TotalTokensServed, &row.TotalTokensSaved,
			&row.CostServedUSD, &row.CostRawUSD, &row.CostSavedUSD,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetSavingsTimeseries scan: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Add the handler and register the route**

Create `internal/api/mcpusage/breakdowns.go`:

```go
package mcpusage

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	mcppkg "github.com/uigraph/app/internal/mcpusage"
)

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsTimeseries(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.DailySavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"timeseries": rows})
}
```

In `internal/api/mcpusage/handler.go`, add to the `store` interface:

```go
GetSavingsTimeseries(ctx context.Context, orgID, modelID string, since time.Time) ([]mcppkg.DailySavings, error)
```

And add to `Register`, after the existing `Summary` line:

```go
requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/timeseries", h.Timeseries)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./tests/... -run TestMCPSavings_Timeseries -v`

Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/mcpusage/mcpusage.go internal/store/postgres/mcp_usage.go internal/api/mcpusage/handler.go internal/api/mcpusage/breakdowns.go tests/mcp_savings_test.go
git commit -m "feat: add MCP savings daily timeseries endpoint"
```

---

### Task 3: By-tool breakdown endpoint

**Files:**
- Modify: `internal/mcpusage/mcpusage.go` (add `ToolSavings` type + `Store` interface method)
- Modify: `internal/store/postgres/mcp_usage.go` (add `GetSavingsByTool`)
- Modify: `internal/api/mcpusage/handler.go` (extend `store` interface + register route)
- Modify: `internal/api/mcpusage/breakdowns.go` (add `ByTool` handler)
- Modify: `tests/mcp_savings_test.go` (add test)

**Interfaces:**
- Produces: `mcpusage.ToolSavings` (`ToolName string`, `TotalCalls int`, `TokensSaved int`, `CostSavedUSD float64`), and `(d *DB) GetSavingsByTool(ctx, orgID, modelID string, since time.Time) ([]mcpusage.ToolSavings, error)`

- [ ] **Step 1: Write the failing test**

Append to `tests/mcp_savings_test.go`:

```go
func TestMCPSavings_ByTool(t *testing.T) {
	tool := fmt.Sprintf("test-bytool-tool-%d", time.Now().UnixNano())

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": tool, "resourceIds": []string{"svc-1"},
		"modelId": "claude-sonnet-4-6", "tokensServed": 100, "tokensRawEquivalent": 600,
		"tokensSaved": 500, "responseSizeBytes": 2048,
	})

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/by-tool?period=1y", adminToken, nil)
	rows := list(body, "byTool")

	var found M
	for _, row := range rows {
		if str(row, "toolName") == tool {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatalf("expected tool %q in by-tool breakdown", tool)
	}
	if int(found["totalCalls"].(float64)) != 1 {
		t.Fatalf("want totalCalls=1, got %v", found["totalCalls"])
	}
	if int(found["tokensSaved"].(float64)) != 500 {
		t.Fatalf("want tokensSaved=500, got %v", found["tokensSaved"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/... -run TestMCPSavings_ByTool -v`

Expected: FAIL — `GET .../mcp/savings/by-tool` 404s.

- [ ] **Step 3: Add the `ToolSavings` type and `Store` interface method**

In `internal/mcpusage/mcpusage.go`, add:

```go
// ToolSavings is one MCP tool's aggregated usage/cost-savings totals.
type ToolSavings struct {
	ToolName     string  `json:"toolName"`
	TotalCalls   int     `json:"totalCalls"`
	TokensSaved  int     `json:"tokensSaved"`
	CostSavedUSD float64 `json:"costSavedUsd"`
}
```

And add to the `Store` interface:

```go
GetSavingsByTool(ctx context.Context, orgID, modelID string, since time.Time) ([]ToolSavings, error)
```

- [ ] **Step 4: Add the Postgres aggregation function**

In `internal/store/postgres/mcp_usage.go`, add:

```go
func (d *DB) GetSavingsByTool(ctx context.Context, orgID, modelID string, since time.Time) ([]mcpusage.ToolSavings, error) {
	const q = `
		SELECT
		    e.tool_name,
		    COUNT(*)                                                                      AS total_calls,
		    COALESCE(SUM(e.tokens_saved), 0)                                               AS tokens_saved,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0) AS cost_saved_usd
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)
		GROUP BY e.tool_name
		ORDER BY cost_saved_usd DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID, since, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsByTool: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.ToolSavings
	for rows.Next() {
		var row mcpusage.ToolSavings
		if scanErr := rows.Scan(&row.ToolName, &row.TotalCalls, &row.TokensSaved, &row.CostSavedUSD); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetSavingsByTool scan: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Add the handler and register the route**

Append to `internal/api/mcpusage/breakdowns.go`:

```go
func (h *Handler) ByTool(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByTool(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.ToolSavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byTool": rows})
}
```

In `internal/api/mcpusage/handler.go`, add to the `store` interface:

```go
GetSavingsByTool(ctx context.Context, orgID, modelID string, since time.Time) ([]mcppkg.ToolSavings, error)
```

And add to `Register`:

```go
requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-tool", h.ByTool)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./tests/... -run TestMCPSavings_ByTool -v`

Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/mcpusage/mcpusage.go internal/store/postgres/mcp_usage.go internal/api/mcpusage/handler.go internal/api/mcpusage/breakdowns.go tests/mcp_savings_test.go
git commit -m "feat: add MCP savings by-tool breakdown endpoint"
```

---

### Task 4: By-model breakdown endpoint

**Files:**
- Modify: `internal/mcpusage/mcpusage.go` (add `ModelSavings` type + `Store` interface method)
- Modify: `internal/store/postgres/mcp_usage.go` (add `GetSavingsByModel`)
- Modify: `internal/api/mcpusage/handler.go` (extend `store` interface + register route)
- Modify: `internal/api/mcpusage/breakdowns.go` (add `ByModel` handler)
- Modify: `tests/mcp_savings_test.go` (add test)

**Interfaces:**
- Produces: `mcpusage.ModelSavings` (`ModelID string`, `DisplayName string`, `Provider string`, `TotalCalls int`, `TokensSaved int`, `CostSavedUSD float64`), and `(d *DB) GetSavingsByModel(ctx, orgID string, since time.Time) ([]mcpusage.ModelSavings, error)` — note: no `modelID` filter param, since this breakdown inherently spans all models

- [ ] **Step 1: Write the failing test**

Append to `tests/mcp_savings_test.go`:

```go
func TestMCPSavings_ByModel(t *testing.T) {
	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/by-model?period=1y", adminToken, nil)
	beforeCalls := 0
	for _, row := range list(before, "byModel") {
		if str(row, "modelId") == "claude-haiku-4-5" {
			beforeCalls = int(row["totalCalls"].(float64))
		}
	}

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "get_api_spec", "resourceIds": []string{"svc-1"},
		"modelId": "claude-haiku-4-5", "tokensServed": 50, "tokensRawEquivalent": 200,
		"tokensSaved": 150, "responseSizeBytes": 1024,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/by-model?period=1y", adminToken, nil)
	var afterCalls int
	var displayName string
	for _, row := range list(after, "byModel") {
		if str(row, "modelId") == "claude-haiku-4-5" {
			afterCalls = int(row["totalCalls"].(float64))
			displayName = str(row, "displayName")
		}
	}
	if afterCalls != beforeCalls+1 {
		t.Fatalf("want totalCalls to increase by 1, before=%d after=%d", beforeCalls, afterCalls)
	}
	if displayName != "Claude Haiku 4.5" {
		t.Fatalf("want displayName %q, got %q", "Claude Haiku 4.5", displayName)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/... -run TestMCPSavings_ByModel -v`

Expected: FAIL — `GET .../mcp/savings/by-model` 404s.

- [ ] **Step 3: Add the `ModelSavings` type and `Store` interface method**

In `internal/mcpusage/mcpusage.go`, add:

```go
// ModelSavings is one LLM model's aggregated usage/cost-savings totals.
type ModelSavings struct {
	ModelID      string  `json:"modelId"`
	DisplayName  string  `json:"displayName"`
	Provider     string  `json:"provider"`
	TotalCalls   int     `json:"totalCalls"`
	TokensSaved  int     `json:"tokensSaved"`
	CostSavedUSD float64 `json:"costSavedUsd"`
}
```

And add to the `Store` interface:

```go
GetSavingsByModel(ctx context.Context, orgID string, since time.Time) ([]ModelSavings, error)
```

- [ ] **Step 4: Add the Postgres aggregation function**

In `internal/store/postgres/mcp_usage.go`, add:

```go
func (d *DB) GetSavingsByModel(ctx context.Context, orgID string, since time.Time) ([]mcpusage.ModelSavings, error) {
	const q = `
		SELECT
		    e.model_id,
		    COALESCE(m.display_name, e.model_id)                                          AS display_name,
		    COALESCE(m.provider, 'unknown')                                                AS provider,
		    COUNT(*)                                                                       AS total_calls,
		    COALESCE(SUM(e.tokens_saved), 0)                                               AS tokens_saved,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0) AS cost_saved_usd
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		GROUP BY e.model_id, m.display_name, m.provider
		ORDER BY cost_saved_usd DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsByModel: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.ModelSavings
	for rows.Next() {
		var row mcpusage.ModelSavings
		if scanErr := rows.Scan(&row.ModelID, &row.DisplayName, &row.Provider, &row.TotalCalls, &row.TokensSaved, &row.CostSavedUSD); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetSavingsByModel scan: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Add the handler and register the route**

Append to `internal/api/mcpusage/breakdowns.go`:

```go
func (h *Handler) ByModel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByModel(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.ModelSavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byModel": rows})
}
```

In `internal/api/mcpusage/handler.go`, add to the `store` interface:

```go
GetSavingsByModel(ctx context.Context, orgID string, since time.Time) ([]mcppkg.ModelSavings, error)
```

And add to `Register`:

```go
requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-model", h.ByModel)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./tests/... -run TestMCPSavings_ByModel -v`

Expected: `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/mcpusage/mcpusage.go internal/store/postgres/mcp_usage.go internal/api/mcpusage/handler.go internal/api/mcpusage/breakdowns.go tests/mcp_savings_test.go
git commit -m "feat: add MCP savings by-model breakdown endpoint"
```

---

### Task 5: By-user breakdown endpoint

**Files:**
- Modify: `internal/mcpusage/mcpusage.go` (add `UserSavings` type + `Store` interface method)
- Modify: `internal/store/postgres/mcp_usage.go` (add `GetSavingsByUser`)
- Modify: `internal/api/mcpusage/handler.go` (extend `store` interface + register route)
- Modify: `internal/api/mcpusage/breakdowns.go` (add `ByUser` handler)
- Modify: `tests/mcp_savings_test.go` (add test)

**Interfaces:**
- Produces: `mcpusage.UserSavings` (`UserID *string`, `ServiceAccountID *string`, `TotalCalls int`, `TokensSaved int`, `CostSavedUSD float64`), and `(d *DB) GetSavingsByUser(ctx, orgID, modelID string, since time.Time) ([]mcpusage.UserSavings, error)` — exactly one of `UserID`/`ServiceAccountID` is non-nil per row, mirroring `UsageEvent`'s existing nullable-pair convention

- [ ] **Step 1: Write the failing test**

Append to `tests/mcp_savings_test.go`:

```go
func TestMCPSavings_ByUser(t *testing.T) {
	me := mustDo(t, "GET", "/api/v1/auth/me", adminToken, nil)
	userID := str(me, "userId")

	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/by-user?period=1y", adminToken, nil)
	beforeCalls := 0
	for _, row := range list(before, "byUser") {
		if str(row, "userId") == userID {
			beforeCalls = int(row["totalCalls"].(float64))
		}
	}

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "list_diagrams", "resourceIds": []string{"svc-1"},
		"modelId": "claude-sonnet-4-6", "tokensServed": 80, "tokensRawEquivalent": 300,
		"tokensSaved": 220, "responseSizeBytes": 900,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/by-user?period=1y", adminToken, nil)
	afterCalls := 0
	for _, row := range list(after, "byUser") {
		if str(row, "userId") == userID {
			afterCalls = int(row["totalCalls"].(float64))
		}
	}
	if afterCalls != beforeCalls+1 {
		t.Fatalf("want totalCalls to increase by 1, before=%d after=%d", beforeCalls, afterCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/... -run TestMCPSavings_ByUser -v`

Expected: FAIL — `GET .../mcp/savings/by-user` 404s.

- [ ] **Step 3: Add the `UserSavings` type and `Store` interface method**

In `internal/mcpusage/mcpusage.go`, add:

```go
// UserSavings is one user's (or service account's) aggregated usage/cost-savings
// totals. Exactly one of UserID/ServiceAccountID is non-nil per row.
type UserSavings struct {
	UserID           *string `json:"userId,omitempty"`
	ServiceAccountID *string `json:"serviceAccountId,omitempty"`
	TotalCalls       int     `json:"totalCalls"`
	TokensSaved      int     `json:"tokensSaved"`
	CostSavedUSD     float64 `json:"costSavedUsd"`
}
```

And add to the `Store` interface:

```go
GetSavingsByUser(ctx context.Context, orgID, modelID string, since time.Time) ([]UserSavings, error)
```

- [ ] **Step 4: Add the Postgres aggregation function**

In `internal/store/postgres/mcp_usage.go`, add:

```go
func (d *DB) GetSavingsByUser(ctx context.Context, orgID, modelID string, since time.Time) ([]mcpusage.UserSavings, error) {
	const q = `
		SELECT
		    e.user_id,
		    e.service_account_id,
		    COUNT(*)                                                                       AS total_calls,
		    COALESCE(SUM(e.tokens_saved), 0)                                                AS tokens_saved,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0)  AS cost_saved_usd
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)
		GROUP BY e.user_id, e.service_account_id
		ORDER BY cost_saved_usd DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID, since, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsByUser: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.UserSavings
	for rows.Next() {
		var row mcpusage.UserSavings
		if scanErr := rows.Scan(&row.UserID, &row.ServiceAccountID, &row.TotalCalls, &row.TokensSaved, &row.CostSavedUSD); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetSavingsByUser scan: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
```

(Scanning directly into `&row.UserID`/`&row.ServiceAccountID`, both `*string` fields, mirrors the existing pattern already used in `ListUsageEvents` for the same nullable columns.)

- [ ] **Step 5: Add the handler and register the route**

Append to `internal/api/mcpusage/breakdowns.go`:

```go
func (h *Handler) ByUser(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByUser(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.UserSavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byUser": rows})
}
```

In `internal/api/mcpusage/handler.go`, add to the `store` interface:

```go
GetSavingsByUser(ctx context.Context, orgID, modelID string, since time.Time) ([]mcppkg.UserSavings, error)
```

And add to `Register`:

```go
requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-user", h.ByUser)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./tests/... -run TestMCPSavings_ByUser -v`

Expected: `PASS`.

- [ ] **Step 7: Run the full test suite one final time**

Run: `go test ./... -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`

Expected: every package `ok`, zero `FAIL` lines.

- [ ] **Step 8: Commit**

```bash
git add internal/mcpusage/mcpusage.go internal/store/postgres/mcp_usage.go internal/api/mcpusage/handler.go internal/api/mcpusage/breakdowns.go tests/mcp_savings_test.go
git commit -m "feat: add MCP savings by-user breakdown endpoint"
```

---

## Summary of new/changed REST contract (for the uigraph-graphql plan)

| Endpoint | Query params | Response envelope |
|---|---|---|
| `GET /api/v1/orgs/{orgID}/mcp/savings/summary` | `period`, `model_id` (optional) | `SavingsSummary` object (unchanged shape, `modelId` now `""` when blended) |
| `GET /api/v1/orgs/{orgID}/mcp/savings/timeseries` | `period`, `model_id` (optional) | `{"timeseries": [DailySavings, ...]}` |
| `GET /api/v1/orgs/{orgID}/mcp/savings/by-tool` | `period`, `model_id` (optional) | `{"byTool": [ToolSavings, ...]}` |
| `GET /api/v1/orgs/{orgID}/mcp/savings/by-model` | `period` | `{"byModel": [ModelSavings, ...]}` |
| `GET /api/v1/orgs/{orgID}/mcp/savings/by-user` | `period`, `model_id` (optional) | `{"byUser": [UserSavings, ...]}` |
