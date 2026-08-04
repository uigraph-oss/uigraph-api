package billing

import "testing"

func TestMatchTags(t *testing.T) {
	rules := []TagRule{
		{TagKey: "team", TagValue: "checkout"},
		{TagKey: "cost-center", TagValue: "payments"},
	}

	tests := []struct {
		name string
		tags map[string]string
		want int
	}{
		{"matches one rule", map[string]string{"team": "checkout", "env": "prod"}, 1},
		{"matches both rules", map[string]string{"team": "checkout", "cost-center": "payments"}, 2},
		{"no match on value", map[string]string{"team": "other"}, 0},
		{"no match on key", map[string]string{"owner": "checkout"}, 0},
		{"empty tags", map[string]string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchTags(tt.tags, rules)
			if len(got) != tt.want {
				t.Errorf("MatchTags(%v) = %v, want %d matches", tt.tags, got, tt.want)
			}
		})
	}
}

func TestFilterMatched(t *testing.T) {
	rules := []TagRule{{TagKey: "team", TagValue: "checkout"}}
	resources := []Resource{
		{ID: "1", Tags: map[string]string{"team": "checkout"}},
		{ID: "2", Tags: map[string]string{"team": "other"}},
	}

	got := FilterMatched(resources, rules)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("FilterMatched() = %+v, want only resource 1", got)
	}
	if len(got[0].MatchedTags) != 1 || got[0].MatchedTags[0] != "team:checkout" {
		t.Fatalf("MatchedTags = %v, want [team:checkout]", got[0].MatchedTags)
	}

	if got := FilterMatched(resources, nil); got != nil {
		t.Fatalf("FilterMatched with no rules = %v, want nil", got)
	}
}

func TestComputeSummary(t *testing.T) {
	resources := []Resource{
		{Provider: ProviderAWS, ResourceType: "compute", MonthlyCostUSD: 100},
		{Provider: ProviderGCP, ResourceType: "database", MonthlyCostUSD: 250},
	}
	s := ComputeSummary(resources, nil)
	if s.TotalMonthlyCostUSD != 350 {
		t.Errorf("TotalMonthlyCostUSD = %v, want 350", s.TotalMonthlyCostUSD)
	}
	if s.ResourceCount != 2 || s.ProviderCount != 2 {
		t.Errorf("ResourceCount/ProviderCount = %d/%d, want 2/2", s.ResourceCount, s.ProviderCount)
	}
	if s.TopCostDriverLabel != "database" || s.TopCostDriverUSD != 250 {
		t.Errorf("TopCostDriver = %s/%v, want database/250", s.TopCostDriverLabel, s.TopCostDriverUSD)
	}
}
