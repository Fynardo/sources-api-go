package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cost "github.com/aws/aws-sdk-go-v2/service/costandusagereportservice"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	l "github.com/RedHatInsights/sources-api-go/logger"
)

// S3API defines the interface for S3 operations
type S3API interface {
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	PutBucketPolicy(ctx context.Context, params *s3.PutBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.PutBucketPolicyOutput, error)
	ListObjects(ctx context.Context, params *s3.ListObjectsInput, optFns ...func(*s3.Options)) (*s3.ListObjectsOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// IAMAPI defines the interface for IAM operations
type IAMAPI interface {
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	DeleteRole(ctx context.Context, params *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
	DeletePolicy(ctx context.Context, params *iam.DeletePolicyInput, optFns ...func(*iam.Options)) (*iam.DeletePolicyOutput, error)
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
	DetachRolePolicy(ctx context.Context, params *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
}

// CostAndUsageReportAPI defines the interface for Cost and Usage Report operations
type CostAndUsageReportAPI interface {
	PutReportDefinition(ctx context.Context, params *cost.PutReportDefinitionInput, optFns ...func(*cost.Options)) (*cost.PutReportDefinitionOutput, error)
	DeleteReportDefinition(ctx context.Context, params *cost.DeleteReportDefinitionInput, optFns ...func(*cost.Options)) (*cost.DeleteReportDefinitionOutput, error)
}

// Client holds the AWS service clients needed for superkey operations
type Client struct {
	AccessKey     string
	SecretKey     string
	Credentials   *aws.Config
	IAM           IAMAPI
	S3            S3API
	CostReporting CostAndUsageReportAPI
}

// NewClient creates a new AWS client with the specified credentials and API clients
func NewClient(ctx context.Context, key, secret string, apis ...string) (*Client, error) {
	client := &Client{
		AccessKey: key,
		SecretKey: secret,
	}

	cfg, err := newAWSConfig(key, secret)
	if err != nil {
		return nil, err
	}

	client.Credentials = cfg

	// Initialize only the required API clients based on the steps
	for _, api := range getRequiredAPIs(apis) {
		switch api {
		case "s3":
			if client.S3 == nil {
				client.S3 = s3.NewFromConfig(*cfg)
			}
		case "iam":
			if client.IAM == nil {
				client.IAM = iam.NewFromConfig(*cfg)
			}
		case "cost_report":
			if client.CostReporting == nil {
				client.CostReporting = cost.NewFromConfig(*cfg)
			}
		default:
			l.Log.Warnf("Unsupported API %q requested when creating Amazon client", api)
		}
	}

	return client, nil
}

// newAWSConfig creates an AWS config with the specified credentials
func newAWSConfig(key, secret string) (*aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     key,
				SecretAccessKey: secret,
				Source:          "SourcesSuperkey",
			},
		}))

	if err != nil {
		return nil, err
	}

	// Default to us-east-1
	cfg.Region = "us-east-1"

	return &cfg, nil
}

// getRequiredAPIs determines which AWS APIs are needed based on the steps
func getRequiredAPIs(steps []string) []string {
	apiSet := make(map[string]bool)

	for _, step := range steps {
		switch step {
		case "s3":
			apiSet["s3"] = true
		case "role", "policy", "bind_role":
			apiSet["iam"] = true
		case "cost_report":
			apiSet["cost_report"] = true
		}
	}

	// Convert set to slice
	apis := make([]string, 0, len(apiSet))
	for api := range apiSet {
		apis = append(apis, api)
	}

	return apis
}
