package amazon

import (
	costtypes "github.com/aws/aws-sdk-go-v2/service/costandusagereportservice/types"
)

// CostS3Policy is the S3 bucket policy required for AWS Cost and Usage Reports
var CostS3Policy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "billingreports.amazonaws.com"
      },
      "Action": [
        "s3:GetBucketAcl",
        "s3:GetBucketPolicy"
      ],
      "Resource": "arn:aws:s3:::S3BUCKET"
    },
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "billingreports.amazonaws.com"
      },
      "Action": "s3:PutObject",
      "Resource": "arn:aws:s3:::S3BUCKET/*"
    }
  ]
}`

// CostReport represents the configuration for an AWS Cost and Usage Report
type CostReport struct {
	AdditionalArtifacts      []costtypes.AdditionalArtifact `json:"additional_artifacts"`
	AdditionalSchemaElements []costtypes.SchemaElement      `json:"additional_schema_elements"`
	Compression              costtypes.CompressionFormat    `json:"compression"`
	Format                   costtypes.ReportFormat         `json:"format"`
	TimeUnit                 costtypes.TimeUnit             `json:"time_unit"`
	ReportName               string                         `json:"report_name"`
	S3Prefix                 string                         `json:"s3_prefix"`
	S3Region                 costtypes.AWSRegion            `json:"s3_region"`
	S3Bucket                 string                         `json:"s3_bucket"`
}
