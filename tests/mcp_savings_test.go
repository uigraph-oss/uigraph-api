// Package tests — see api_test.go for shared TestMain setup, adminToken, orgID.
package tests

import (
	"fmt"
	"testing"
	"time"
)

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
