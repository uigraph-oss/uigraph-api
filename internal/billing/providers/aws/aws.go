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
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/uigraph/app/internal/billing"
)

const syncWindowDays = 30

// defaultRegion is used only when AWS_REGION isn't configured at all — STS
// and Cost Explorer both need some region to resolve their endpoint, even
// though neither is meaningfully region-scoped. Resource discovery ignores
// this entirely and scans every region regardless (see
// discoverResourcesAllRegions).
const defaultRegion = "us-east-1"

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

// assumedConfig assumes the customer's role (RoleARN + ExternalID from
// `in`) and returns an aws.Config scoped to it, with its region resolved
// from AWS_REGION (falling back to defaultRegion if unset).
func (a *Adapter) assumedConfig(ctx context.Context, in billing.ConnectionInput) (awssdk.Config, error) {
	if in.RoleARN == "" || in.ExternalID == "" {
		return awssdk.Config{}, billing.ErrInvalidCredential
	}

	base, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("aws: load base config: %w", err)
	}
	region := base.Region
	if region == "" {
		region = defaultRegion
	}

	stsClient := sts.NewFromConfig(base, func(o *sts.Options) {
		o.Region = region
	})
	provider := stscreds.NewAssumeRoleProvider(stsClient, in.RoleARN, func(o *stscreds.AssumeRoleOptions) {
		o.ExternalID = awssdk.String(in.ExternalID)
		o.RoleSessionName = "uigraph-billing-sync"
	})
	assumed := base.Copy()
	assumed.Region = region
	assumed.Credentials = awssdk.NewCredentialsCache(provider)
	return assumed, nil
}

func costExplorerClient(cfg awssdk.Config) *costexplorer.Client {
	return costexplorer.NewFromConfig(cfg)
}

// TestConnection verifies the role can be assumed and can read cost data,
// without persisting anything.
func (a *Adapter) TestConnection(ctx context.Context, in billing.ConnectionInput) error {
	cfg, err := a.assumedConfig(ctx, in)
	if err != nil {
		return err
	}
	ce := costExplorerClient(cfg)

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

// Sync discovers tagged resources across every AWS region (infrastructure
// isn't confined to whatever region happens to be configured) and their
// cost over the last syncWindowDays. Resource Groups Tagging gives exact
// resource inventory and tags; Cost Explorer's public API has no true
// per-resource cost grouping, so daily/monthly cost is allocated evenly
// across resources that share the same AWS service (see allocateCosts) —
// an approximation, not exact resource-level billing.
func (a *Adapter) Sync(ctx context.Context, in billing.ConnectionInput) (billing.SyncResult, error) {
	cfg, err := a.assumedConfig(ctx, in)
	if err != nil {
		return billing.SyncResult{}, err
	}

	resources, err := discoverResourcesAllRegions(ctx, cfg)
	if err != nil {
		return billing.SyncResult{}, err
	}

	dailyServiceCosts, err := fetchDailyCostsByService(ctx, costExplorerClient(cfg), syncWindowDays)
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
