package billing

// MatchTags reports which of a resource's tags satisfy any of the given tag
// rules, and whether at least one rule matched. A resource is considered
// linked to a service exactly when len(matched) > 0.
func MatchTags(tags map[string]string, rules []TagRule) []string {
	matched := make([]string, 0, len(rules))
	for _, rule := range rules {
		if v, ok := tags[rule.TagKey]; ok && v == rule.TagValue {
			matched = append(matched, rule.TagKey+":"+rule.TagValue)
		}
	}
	return matched
}

// FilterMatched returns the subset of resources that match at least one of
// rules, with MatchedTags populated on each.
func FilterMatched(resources []Resource, rules []TagRule) []Resource {
	if len(rules) == 0 {
		return nil
	}
	out := make([]Resource, 0, len(resources))
	for _, r := range resources {
		matched := MatchTags(r.Tags, rules)
		if len(matched) == 0 {
			continue
		}
		r.MatchedTags = matched
		out = append(out, r)
	}
	return out
}

// ComputeSummary derives the KPI row from a service's matched resources and
// its two most recent trend points (for month-over-month change).
func ComputeSummary(resources []Resource, trend []TrendPoint) Summary {
	var s Summary
	providers := make(map[Provider]bool)
	typeCosts := make(map[string]float64)

	for _, r := range resources {
		s.TotalMonthlyCostUSD += r.MonthlyCostUSD
		providers[r.Provider] = true
		typeCosts[r.ResourceType] += r.MonthlyCostUSD
	}
	s.ResourceCount = len(resources)
	s.ProviderCount = len(providers)

	for resourceType, cost := range typeCosts {
		if cost > s.TopCostDriverUSD {
			s.TopCostDriverUSD = cost
			s.TopCostDriverLabel = resourceType
		}
	}

	if len(trend) >= 60 {
		current := sumRange(trend[len(trend)-30:])
		previous := sumRange(trend[len(trend)-60 : len(trend)-30])
		if previous > 0 {
			s.MoMChangePct = (current - previous) / previous * 100
		}
	}

	return s
}

func sumRange(points []TrendPoint) float64 {
	var total float64
	for _, p := range points {
		total += p.TotalUSD
	}
	return total
}
