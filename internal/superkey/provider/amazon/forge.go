package amazon

import (
	
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"

	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
	l "github.com/RedHatInsights/sources-api-go/logger"
)

// AmazonProvider implements the Provider interface for Amazon Web Services
type AmazonProvider struct {
	Client *Client
}

// NewAmazonProvider creates a new AmazonProvider instance
func NewAmazonProvider(client *Client) *AmazonProvider {
	return &AmazonProvider{
		Client: client,
	}
}

// ForgeApplication creates AWS resources based on the superkey request
// Returns a Result containing information about created resources
func (a *AmazonProvider) ForgeApplication(ctx context.Context, request *provider.CreateRequest) (*provider.Result, error) {
	// Generate a unique GUID for this application's resources
	guid, err := generateGUID()
	if err != nil {
		return nil, fmt.Errorf("unable to generate GUID: %w", err)
	}

	result := &provider.Result{
		StepsCompleted: make(map[string]map[string]string),
		Request:        request,
		GUID:           guid,
	}

	// Process each step in order
	for _, step := range request.SuperKeySteps {
		switch step.Name {
		case "s3":
			if err := a.processS3Step(ctx, step, result); err != nil {
				return result, err
			}

		case "cost_report":
			if err := a.processCostReportStep(ctx, step, result); err != nil {
				return result, err
			}

		case "policy":
			if err := a.processPolicyStep(ctx, step, result); err != nil {
				return result, err
			}

		case "role":
			if err := a.processRoleStep(ctx, step, result); err != nil {
				return result, err
			}

		case "bind_role":
			if err := a.processBindRoleStep(ctx, step, result); err != nil {
				return result, err
			}

		default:
			return result, provider.WrapError(step.Name, fmt.Sprintf("superkey step %q not implemented", step.Name), nil)
		}
	}

	// Set authentication details from the completed role
	if result.StepsCompleted["role"] != nil {
		result.RoleARN = result.StepsCompleted["role"]["arn"]
		if authType, ok := result.Request.Extra["result_type"]; ok {
			result.AuthType = authType
		}
	}

	return result, nil
}

// processS3Step handles the S3 bucket creation step
func (a *AmazonProvider) processS3Step(ctx context.Context, step provider.Step, result *provider.Result) error {
	name := fmt.Sprintf("%s-bucket-%s", getShortName(result.Request.ApplicationType), result.GUID)

	l.Log.Infof("Creating S3 bucket %q", name)

	err := a.Client.CreateS3Bucket(ctx, name)
	if err != nil {
		return provider.WrapError("s3", fmt.Sprintf("failed to create S3 bucket %q", name), err)
	}

	result.MarkCompleted("s3", map[string]string{"output": name})
	l.Log.Infof("S3 bucket %q created successfully", name)

	// If this S3 bucket needs a cost reporting policy, attach it
	if step.Payload == `"create_cost_policy"` {
		l.Log.Debugf("Attaching cost policy to S3 bucket %q", name)

		policy := substituteInPayload(CostS3Policy, result, step.Substitutions)

		err := a.Client.AttachBucketPolicy(ctx, name, policy)
		if err != nil {
			return provider.WrapError("s3", fmt.Sprintf("failed to attach bucket policy to S3 bucket %q", name), err)
		}

		l.Log.Infof("S3 bucket policy attached to bucket %q", name)
	}

	return nil
}

// processCostReportStep handles the Cost and Usage Report creation step
func (a *AmazonProvider) processCostReportStep(ctx context.Context, step provider.Step, result *provider.Result) error {
	payload := substituteInPayload(step.Payload, result, step.Substitutions)

	var costReport CostReport
	err := json.Unmarshal([]byte(payload), &costReport)
	if err != nil {
		return provider.WrapError("cost_report", fmt.Sprintf("failed to parse cost report payload: %s", payload), err)
	}

	// Append GUID to make the report name unique
	costReport.ReportName = fmt.Sprintf("%s-%s", costReport.ReportName, result.GUID)

	l.Log.Infof("Creating cost and usage report %q", costReport.ReportName)

	err = a.Client.CreateCostAndUsageReport(ctx, &costReport)
	if err != nil {
		return provider.WrapError("cost_report", fmt.Sprintf("failed to create cost and usage report %q", costReport.ReportName), err)
	}

	result.MarkCompleted("cost_report", map[string]string{"output": costReport.ReportName})
	l.Log.Infof("Cost and usage report %q created successfully", costReport.ReportName)

	return nil
}

// processPolicyStep handles the IAM policy creation step
func (a *AmazonProvider) processPolicyStep(ctx context.Context, step provider.Step, result *provider.Result) error {
	name := fmt.Sprintf("%s-policy-%s", getShortName(result.Request.ApplicationType), result.GUID)
	payload := substituteInPayload(step.Payload, result, step.Substitutions)

	l.Log.Infof("Creating IAM policy %q", name)

	arn, err := a.Client.CreatePolicy(ctx, name, payload)
	if err != nil {
		return provider.WrapError("policy", fmt.Sprintf("failed to create policy %q", name), err)
	}

	result.MarkCompleted("policy", map[string]string{"output": *arn})
	l.Log.Infof("IAM policy %q created successfully", name)

	return nil
}

// processRoleStep handles the IAM role creation step
func (a *AmazonProvider) processRoleStep(ctx context.Context, step provider.Step, result *provider.Result) error {
	name := fmt.Sprintf("%s-role-%s", getShortName(result.Request.ApplicationType), result.GUID)
	payload := substituteInPayload(step.Payload, result, step.Substitutions)

	l.Log.Infof("Creating IAM role %q", name)

	roleArn, err := a.Client.CreateRole(ctx, name, payload)
	if err != nil {
		return provider.WrapError("role", fmt.Sprintf("failed to create role %q", name), err)
	}

	// Store both the role name and ARN
	result.MarkCompleted("role", map[string]string{
		"output": name,
		"arn":    *roleArn,
	})
	l.Log.Infof("IAM role %q created successfully with ARN %s", name, *roleArn)

	return nil
}

// processBindRoleStep handles attaching a policy to a role
func (a *AmazonProvider) processBindRoleStep(ctx context.Context, step provider.Step, result *provider.Result) error {
	roleName := result.StepsCompleted["role"]["output"]
	policyArn := result.StepsCompleted["policy"]["output"]

	l.Log.Infof("Binding policy %q to role %q", policyArn, roleName)

	err := a.Client.BindPolicyToRole(ctx, policyArn, roleName)
	if err != nil {
		return provider.WrapError("bind_role", fmt.Sprintf("failed to bind policy %q to role %q", policyArn, roleName), err)
	}

	result.MarkCompleted("bind_role", map[string]string{})
	l.Log.Infof("Policy %q bound to role %q successfully", policyArn, roleName)

	return nil
}

// generateGUID generates a unique identifier for the resources
func generateGUID() (string, error) {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getShortName generates a standardized resource name prefix from the application type
func getShortName(applicationTypeName string) string {
	return fmt.Sprintf("redhat-%s", path.Base(applicationTypeName))
}
