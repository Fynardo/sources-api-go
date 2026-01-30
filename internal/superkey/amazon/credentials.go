package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// NewAmazonConfig returns an AWS config struct with access key + secret + region set
func NewAmazonConfig(ctx context.Context, key, secret string) (*aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     key,
				SecretAccessKey: secret,
				Source:          "SourcesApiSuperkey",
			},
		}))

	if err != nil {
		return nil, err
	}

	// Defaulting to us-east-1 for now
	cfg.Region = "us-east-1"

	return &cfg, nil
}
