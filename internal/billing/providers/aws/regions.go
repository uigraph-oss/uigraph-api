package aws

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
)

// regionConcurrency bounds how many regions are scanned in parallel, so a
// manual "sync now" stays fast without hammering the account with 30+
// simultaneous API calls.
const regionConcurrency = 8

// allCommercialRegions is every standard AWS commercial-partition region.
// Resource discovery is scanned across all of them rather than a single
// configured region — infrastructure isn't confined to one region, and the
// UI already offers a region filter. Regions the account hasn't opted into
// simply error and are skipped (see discoverResourcesAllRegions).
var allCommercialRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"af-south-1",
	"ap-east-1", "ap-south-1", "ap-south-2",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
	"ca-central-1", "ca-west-1",
	"eu-central-1", "eu-central-2",
	"eu-west-1", "eu-west-2", "eu-west-3",
	"eu-north-1", "eu-south-1", "eu-south-2",
	"il-central-1",
	"me-south-1", "me-central-1",
	"sa-east-1",
}

// discoverResourcesAllRegions fans out discoverResourcesInRegion across
// every commercial region concurrently (bounded by regionConcurrency).
// Regions the account isn't opted into (or that otherwise error) are
// logged and skipped rather than failing the whole sync — most accounts
// only use a handful of the 25+ regions that exist. Results are
// deduplicated by ARN since a handful of resource types (e.g. S3 buckets)
// are technically global and can surface under more than one region's
// tagging-API endpoint.
func discoverResourcesAllRegions(ctx context.Context, cfg aws.Config) ([]awsResource, error) {
	type regionResult struct {
		region    string
		resources []awsResource
		err       error
	}

	results := make(chan regionResult, len(allCommercialRegions))
	sem := make(chan struct{}, regionConcurrency)
	var wg sync.WaitGroup

	for _, region := range allCommercialRegions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			client := resourcegroupstaggingapi.NewFromConfig(cfg, func(o *resourcegroupstaggingapi.Options) {
				o.Region = region
			})
			resources, err := discoverResourcesInRegion(ctx, client)
			results <- regionResult{region: region, resources: resources, err: err}
		}(region)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	seen := make(map[string]bool)
	var out []awsResource
	var firstErr error
	succeeded := 0
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			slog.DebugContext(ctx, "aws: skipping region (not opted-in or inaccessible)", "region", res.region, "err", res.err)
			continue
		}
		succeeded++
		for _, r := range res.resources {
			if seen[r.ExternalResourceID] {
				continue
			}
			seen[r.ExternalResourceID] = true
			out = append(out, r)
		}
	}

	// A handful of region failures is normal (most accounts aren't opted
	// into all 25+ commercial regions). Every single region failing is not
	// — that points at a real credential/permission problem, which should
	// surface as a sync error rather than a silent "0 resources" success.
	if succeeded == 0 {
		return nil, fmt.Errorf("aws: GetResources failed in every region: %w", firstErr)
	}

	return out, nil
}
