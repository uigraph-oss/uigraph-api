// Package aws implements billing.ProviderAdapter against AWS Cost Explorer
// and the Resource Groups Tagging API, authenticating by assuming a
// customer-granted cross-account IAM role (no long-lived AWS access keys
// are ever stored).
package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/uigraph/app/internal/billing"
)

const syncWindowDays = 30

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

// clients assumes the customer's role (RoleARN + ExternalID from `in`) and
// returns Cost Explorer + Resource Groups Tagging clients scoped to it.
func (a *Adapter) clients(ctx context.Context, in billing.ConnectionInput) (*costexplorer.Client, *resourcegroupstaggingapi.Client, error) {
	if in.RoleARN == "" || in.ExternalID == "" {
		return nil, nil, billing.ErrInvalidCredential
	}

	base, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("aws: load base config: %w", err)
	}

	stsClient := sts.NewFromConfig(base)
	provider := stscreds.NewAssumeRoleProvider(stsClient, in.RoleARN, func(o *stscreds.AssumeRoleOptions) {
		o.ExternalID = awssdk.String(in.ExternalID)
		o.RoleSessionName = "uigraph-billing-sync"
	})
	assumed := base.Copy()
	assumed.Credentials = awssdk.NewCredentialsCache(provider)

	return costexplorer.NewFromConfig(assumed), resourcegroupstaggingapi.NewFromConfig(assumed), nil
}

// TestConnection verifies the role can be assumed and can read cost data,
// without persisting anything.
func (a *Adapter) TestConnection(ctx context.Context, in billing.ConnectionInput) error {
	ce, _, err := a.clients(ctx, in)
	if err != nil {
		return err
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -1)
	_, err = ce.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: awssdk.String(start.Format("2006-01-02")),
			End:   awssdk.String(end.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	})
	if err != nil {
		return fmt.Errorf("aws: test connection: %w", err)
	}
	return nil
}

// Sync discovers tagged resources and their cost over the last
// syncWindowDays. Resource Groups Tagging gives exact resource inventory
// and tags; Cost Explorer's public API has no true per-resource cost
// grouping, so daily/monthly cost is allocated evenly across resources that
// share the same AWS service (see allocateCosts) — an approximation, not
// exact resource-level billing.
func (a *Adapter) Sync(ctx context.Context, in billing.ConnectionInput) (billing.SyncResult, error) {
	ce, tagging, err := a.clients(ctx, in)
	if err != nil {
		return billing.SyncResult{}, err
	}

	resources, err := discoverResources(ctx, tagging)
	if err != nil {
		return billing.SyncResult{}, err
	}

	dailyServiceCosts, err := fetchDailyCostsByService(ctx, ce, syncWindowDays)
	if err != nil {
		return billing.SyncResult{}, err
	}

	usage := allocateCosts(resources, dailyServiceCosts)

	monthly := make(map[string]float64, len(resources))
	for _, u := range usage {
		monthly[u.ExternalResourceID] += u.CostUSD
	}

	out := make([]billing.Resource, len(resources))
	for i, r := range resources {
		out[i] = r.Resource
		out[i].MonthlyCostUSD = monthly[r.ExternalResourceID] * (30.0 / syncWindowDays)
	}

	return billing.SyncResult{Resources: out, Usage: usage}, nil
}
