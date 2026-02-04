package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cost "github.com/aws/aws-sdk-go-v2/service/costandusagereportservice"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	l "github.com/RedHatInsights/sources-api-go/logger"
)

// AmazonClient is the interface for AWS operations (for mocking in tests)
type AmazonClient interface {
	CreateRole(ctx context.Context, name, payload string) (*string, error)
	DestroyRole(ctx context.Context, name string) error
	CreatePolicy(ctx context.Context, name, payload string) (*string, error)
	DestroyPolicy(ctx context.Context, arn string) error
	BindPolicyToRole(ctx context.Context, policy, role string) error
	UnBindPolicyToRole(ctx context.Context, policy, role string) error
	CreateS3Bucket(ctx context.Context, name string) error
	DestroyS3Bucket(ctx context.Context, name string) error
	AttachBucketPolicy(ctx context.Context, bucket, policy string) error
	CreateCostAndUsageReport(ctx context.Context, report *CostReport) error
	DestroyCostAndUsageReport(ctx context.Context, name string) error
}

// Client is the Amazon client struct that implements AmazonClient
type Client struct {
	AccessKey     string
	SecretKey     string
	Credentials   *aws.Config
	Iam           *iam.Client
	S3            *s3.Client
	CostReporting *cost.Client
}

// NewClient creates a new Amazon client with the specified API clients
func NewClient(ctx context.Context, key, secret string, apis ...string) (*Client, error) {
	a := &Client{AccessKey: key, SecretKey: secret}

	creds, err := NewAmazonConfig(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	a.Credentials = creds

	for _, api := range getRequiredApis(apis) {
		switch api {
		case "s3":
			if a.S3 == nil {
				a.S3 = s3.NewFromConfig(*creds)
			}
		case "iam":
			if a.Iam == nil {
				a.Iam = iam.NewFromConfig(*creds)
			}
		case "cost_report":
			if a.CostReporting == nil {
				a.CostReporting = cost.NewFromConfig(*creds)
			}
		default:
			l.Log.Errorf(`Unsupported "%s" API requested when creating an Amazon client`, api)
		}
	}

	return a, nil
}

// getRequiredApis maps step names to required AWS API clients
func getRequiredApis(steps []string) []string {
	apis := make([]string, 0)
	for _, step := range steps {
		switch step {
		case "s3":
			apis = append(apis, "s3")
		case "role", "policy", "bind_role":
			apis = append(apis, "iam")
		case "cost_report":
			apis = append(apis, "cost_report")
		}
	}

	return apis
}
