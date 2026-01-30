package amazon

import (
	"context"

	cost "github.com/aws/aws-sdk-go-v2/service/costandusagereportservice"
	"github.com/aws/aws-sdk-go-v2/service/costandusagereportservice/types"
)

// CreateCostAndUsageReport creates an AWS Cost and Usage Report
func (c *Client) CreateCostAndUsageReport(ctx context.Context, costReport *CostReport) error {
	reportDefinition := &types.ReportDefinition{
		AdditionalSchemaElements: costReport.AdditionalSchemaElements,
		Compression:              costReport.Compression,
		Format:                   costReport.Format,
		ReportName:               &costReport.ReportName,
		S3Bucket:                 &costReport.S3Bucket,
		S3Prefix:                 &costReport.S3Prefix,
		S3Region:                 costReport.S3Region,
		TimeUnit:                 costReport.TimeUnit,
		AdditionalArtifacts:      costReport.AdditionalArtifacts,
	}

	_, err := c.CostReporting.PutReportDefinition(ctx, &cost.PutReportDefinitionInput{
		ReportDefinition: reportDefinition,
	})
	return err
}

// DestroyCostAndUsageReport deletes an AWS Cost and Usage Report
func (c *Client) DestroyCostAndUsageReport(ctx context.Context, name string) error {
	_, err := c.CostReporting.DeleteReportDefinition(ctx, &cost.DeleteReportDefinitionInput{
		ReportName: &name,
	})
	return err
}
