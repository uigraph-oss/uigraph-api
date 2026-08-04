package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"

	"github.com/uigraph/app/internal/billing"
)

// awsResource is an intermediate representation carrying the AWS "service"
// segment of the resource ARN (e.g. "ec2", "rds") alongside the resource,
// used later to allocate Cost Explorer's per-service totals.
type awsResource struct {
	billing.Resource
	ceServiceName string
}

// discoverResourcesInRegion lists every resource carrying at least one tag
// in the region client is bound to, via the Resource Groups Tagging API
// (paginated). This is exact — unlike cost allocation below, resource
// identity and tags come directly from AWS.
func discoverResourcesInRegion(ctx context.Context, client *resourcegroupstaggingapi.Client) ([]awsResource, error) {
	var out []awsResource
	var token *string

	for {
		resp, err := client.GetResources(ctx, &resourcegroupstaggingapi.GetResourcesInput{
			PaginationToken:     token,
			ResourcesPerPage:    awsIntPtr(100),
			ResourceTypeFilters: supportedResourceTypeFilters(),
		})
		if err != nil {
			return nil, fmt.Errorf("aws: GetResources: %w", err)
		}

		for _, m := range resp.ResourceTagMappingList {
			r, ok := toResource(m)
			if !ok {
				continue
			}
			out = append(out, r)
		}

		if resp.PaginationToken == nil || *resp.PaginationToken == "" {
			break
		}
		token = resp.PaginationToken
	}

	return out, nil
}

func awsIntPtr(v int32) *int32 { return &v }

// supportedResourceTypeFilters restricts discovery to the AWS services we
// know how to classify into a billing.ResourceType (see arnServiceInfo) —
// tagged resources of other types are skipped rather than shown unlabeled.
func supportedResourceTypeFilters() []string {
	return []string{
		"ec2:instance", "ec2:volume", "ec2:natgateway", "ec2:vpc",
		"rds:db", "rds:cluster",
		"dynamodb:table",
		"s3",
		"sqs",
		"elasticloadbalancing:loadbalancer",
		"lambda:function",
		"elasticache:cluster",
		"eks:cluster",
	}
}

func toResource(m rgtypes.ResourceTagMapping) (awsResource, bool) {
	arnStr := ""
	if m.ResourceARN != nil {
		arnStr = *m.ResourceARN
	}
	parts := strings.SplitN(arnStr, ":", 6)
	if len(parts) < 6 {
		return awsResource{}, false
	}
	service, region := parts[2], parts[3]
	resourceID := parts[5]

	info, ok := arnServiceInfo(service, resourceID)
	if !ok {
		return awsResource{}, false
	}

	tags := make(map[string]string, len(m.Tags))
	for _, t := range m.Tags {
		if t.Key == nil || t.Value == nil {
			continue
		}
		tags[*t.Key] = *t.Value
	}

	name := resourceID
	if idx := strings.LastIndexAny(resourceID, "/:"); idx >= 0 {
		name = resourceID[idx+1:]
	}
	if v, ok := tags["Name"]; ok && v != "" {
		name = v
	}

	environment := ""
	for _, key := range []string{"environment", "Environment", "env", "Env"} {
		if v, ok := tags[key]; ok {
			environment = strings.ToLower(v)
			break
		}
	}

	return awsResource{
		Resource: billing.Resource{
			ExternalResourceID: arnStr,
			Name:               name,
			ResourceType:       info.resourceType,
			Provider:           billing.ProviderAWS,
			Region:             region,
			Environment:        environment,
			Status:             billing.ResourceStatusRunning,
			Tags:               tags,
		},
		ceServiceName: info.ceServiceName,
	}, true
}

type arnInfo struct {
	resourceType  string
	ceServiceName string
}

// arnServiceInfo maps an ARN's service segment (+ resource-id prefix, where
// one service segment spans multiple resource kinds e.g. "ec2") to our
// ResourceType and to the exact service name Cost Explorer's SERVICE
// dimension uses — the two vocabularies don't match, so this table is the
// bridge between "what AWS resource is this" and "what AWS bills it as".
func arnServiceInfo(service, resourceID string) (arnInfo, bool) {
	prefix := resourceID
	if idx := strings.IndexAny(resourceID, "/:"); idx >= 0 {
		prefix = resourceID[:idx]
	}

	switch service {
	case "ec2":
		switch prefix {
		case "instance":
			return arnInfo{"compute", "Amazon Elastic Compute Cloud - Compute"}, true
		case "volume":
			return arnInfo{"storage", "EC2 - Other"}, true
		case "natgateway", "vpc", "vpc-endpoint", "subnet":
			return arnInfo{"network", "EC2 - Other"}, true
		}
		return arnInfo{}, false
	case "rds":
		return arnInfo{"database", "Amazon Relational Database Service"}, true
	case "dynamodb":
		return arnInfo{"database", "Amazon DynamoDB"}, true
	case "s3":
		return arnInfo{"storage", "Amazon Simple Storage Service"}, true
	case "sqs":
		return arnInfo{"queue", "Amazon Simple Queue Service"}, true
	case "elasticloadbalancing":
		return arnInfo{"load_balancer", "Amazon Elastic Load Balancing"}, true
	case "lambda":
		return arnInfo{"serverless", "AWS Lambda"}, true
	case "elasticache":
		return arnInfo{"cache", "Amazon ElastiCache"}, true
	case "eks":
		return arnInfo{"kubernetes", "Amazon Elastic Kubernetes Service"}, true
	default:
		return arnInfo{}, false
	}
}
