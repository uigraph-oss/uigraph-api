package aws

import (
	"fmt"
	"strconv"
	"time"

	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/uigraph/app/internal/billing"
)

// dailyServiceCosts is date (YYYY-MM-DD) -> AWS service name -> total cost.
type dailyServiceCosts map[string]map[string]float64

// fetchDailyCostsByService is the only real cost signal the public Cost
// Explorer API gives us without a Cost & Usage Report (CUR) export: total
// spend per day, grouped by AWS service. There is no per-resource
// dimension available here.
func fetchDailyCostsByService(ctx context.Context, client *costexplorer.Client, days int) (dailyServiceCosts, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -days)

	out := dailyServiceCosts{}
	var token *string
	for {
		resp, err := client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
			TimePeriod: &cetypes.DateInterval{
				Start: awssdk.String(start.Format("2006-01-02")),
				End:   awssdk.String(end.Format("2006-01-02")),
			},
			Granularity: cetypes.GranularityDaily,
			Metrics:     []string{"UnblendedCost"},
			GroupBy: []cetypes.GroupDefinition{
				{Type: cetypes.GroupDefinitionTypeDimension, Key: awssdk.String("SERVICE")},
			},
			NextPageToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("aws: GetCostAndUsage: %w", err)
		}

		for _, byTime := range resp.ResultsByTime {
			if byTime.TimePeriod == nil || byTime.TimePeriod.Start == nil {
				continue
			}
			date := *byTime.TimePeriod.Start
			for _, group := range byTime.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				serviceName := group.Keys[0]
				metric, ok := group.Metrics["UnblendedCost"]
				if !ok || metric.Amount == nil {
					continue
				}
				amount, err := strconv.ParseFloat(*metric.Amount, 64)
				if err != nil {
					continue
				}
				if out[date] == nil {
					out[date] = map[string]float64{}
				}
				out[date][serviceName] += amount
			}
		}

		if resp.NextPageToken == nil || *resp.NextPageToken == "" {
			break
		}
		token = resp.NextPageToken
	}

	return out, nil
}

// allocateCosts distributes each day's per-service total evenly across the
// discovered resources that belong to that AWS service. This is an
// approximation — the public Cost Explorer API has no per-resource cost
// dimension without an opt-in Cost & Usage Report export — but it is
// derived entirely from real billing totals, not invented numbers, and it
// is the same technique most lightweight cost-visibility tools use absent
// a CUR integration.
func allocateCosts(resources []awsResource, costs dailyServiceCosts) []billing.UsagePoint {
	countByService := map[string]int{}
	for _, r := range resources {
		countByService[r.ceServiceName]++
	}

	var out []billing.UsagePoint
	for date, byService := range costs {
		for serviceName, total := range byService {
			n := countByService[serviceName]
			if n == 0 {
				continue
			}
			perResource := total / float64(n)
			for _, r := range resources {
				if r.ceServiceName != serviceName {
					continue
				}
				out = append(out, billing.UsagePoint{
					ExternalResourceID: r.ExternalResourceID,
					Date:               date,
					CostUSD:            perResource,
				})
			}
		}
	}
	return out
}
