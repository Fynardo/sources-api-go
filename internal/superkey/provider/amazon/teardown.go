package amazon

import (
	
	"context"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
	l "github.com/RedHatInsights/sources-api-go/logger"
)

// TearDown destroys AWS resources that were created during ForgeApplication
// Resources are destroyed in reverse dependency order to avoid conflicts
// Returns a slice of errors if any destruction operations failed
func (a *AmazonProvider) TearDown(ctx context.Context, result *provider.Result) []error {
	errors := make([]error, 0)

	// Step 1: Unbind the policy from the role (if it was bound)
	// This must happen first so we can cleanly delete both the policy and role
	if result.StepsCompleted["bind_role"] != nil {
		policyArn := result.StepsCompleted["policy"]["output"]
		roleName := result.StepsCompleted["role"]["output"]

		l.Log.Infof("Unbinding policy %q from role %q", policyArn, roleName)

		err := a.Client.UnBindPolicyToRole(ctx, policyArn, roleName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to unbind policy %q from role %q: %w", policyArn, roleName, err))
		} else {
			l.Log.Infof("Policy %q unbound from role %q", policyArn, roleName)
		}
	}

	// Step 2: Delete the IAM policy (independent after unbinding)
	if result.StepsCompleted["policy"] != nil {
		policyArn := result.StepsCompleted["policy"]["output"]

		l.Log.Infof("Destroying IAM policy %q", policyArn)

		err := a.Client.DestroyPolicy(ctx, policyArn)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy policy %q: %w", policyArn, err))
		} else {
			l.Log.Infof("IAM policy %q destroyed", policyArn)
		}
	}

	// Step 3: Delete the IAM role (independent after unbinding)
	if result.StepsCompleted["role"] != nil {
		roleName := result.StepsCompleted["role"]["output"]

		l.Log.Infof("Destroying IAM role %q", roleName)

		err := a.Client.DestroyRole(ctx, roleName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy role %q: %w", roleName, err))
		} else {
			l.Log.Infof("IAM role %q destroyed", roleName)
		}
	}

	// Step 4: Delete the Cost and Usage Report (independent)
	if result.StepsCompleted["cost_report"] != nil {
		reportName := result.StepsCompleted["cost_report"]["output"]

		l.Log.Infof("Destroying cost and usage report %q", reportName)

		err := a.Client.DestroyCostAndUsageReport(ctx, reportName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy cost and usage report %q: %w", reportName, err))
		} else {
			l.Log.Infof("Cost and usage report %q destroyed", reportName)
		}
	}

	// Step 5: Delete the S3 bucket (last, in case other resources depend on it)
	if result.StepsCompleted["s3"] != nil {
		bucketName := result.StepsCompleted["s3"]["output"]

		l.Log.Infof("Destroying S3 bucket %q", bucketName)

		err := a.Client.DestroyS3Bucket(ctx, bucketName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy S3 bucket %q: %w", bucketName, err))
		} else {
			l.Log.Infof("S3 bucket %q destroyed", bucketName)
		}
	}

	return errors
}
