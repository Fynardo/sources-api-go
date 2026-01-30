package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	l "github.com/RedHatInsights/sources-api-go/logger"

	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/amazon"
)

// AmazonProvider implements the Provider interface for AWS
type AmazonProvider struct {
	Client amazon.AmazonClient
}

// NewAmazonProvider creates a new Amazon provider with credentials from the superkey auth
func NewAmazonProvider(ctx context.Context, superKeyAuth string) (*AmazonProvider, error) {
	// The superKeyAuth is the authentication ID - we need to fetch the credentials
	// This will be handled by the orchestrator which passes the credentials directly
	return &AmazonProvider{}, nil
}

// NewAmazonProviderWithCredentials creates a new Amazon provider with the given credentials
func NewAmazonProviderWithCredentials(ctx context.Context, accessKey, secretKey string, steps []string) (*AmazonProvider, error) {
	stepNames := make([]string, len(steps))
	for i, step := range steps {
		stepNames[i] = step
	}

	client, err := amazon.NewClient(ctx, accessKey, secretKey, stepNames...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Amazon client: %w", err)
	}

	return &AmazonProvider{Client: client}, nil
}

// ForgeApplication creates AWS resources based on the superkey request steps
func (a *AmazonProvider) ForgeApplication(ctx context.Context, request *superkey.CreateRequest) (*superkey.ForgedApplication, error) {
	guid, err := generateGUID()
	if err != nil {
		return nil, fmt.Errorf("unable to generate guid: %w", err)
	}

	f := &superkey.ForgedApplication{
		StepsCompleted: make(map[string]map[string]string),
		Request:        request,
		Client:         a,
		GUID:           guid,
	}

	for _, step := range request.SuperKeySteps {
		switch step.Name {
		case "s3":
			name := fmt.Sprintf("%v-bucket-%v", getShortName(f.Request.ApplicationType), f.GUID)

			err := a.Client.CreateS3Bucket(ctx, name)
			if err != nil {
				return f, superkey.NewForgeError("s3", name, fmt.Errorf("failed to create S3 bucket: %w", err), f)
			}

			f.StepsCompleted["s3"] = map[string]string{"output": name}
			l.Log.Infof(`S3 bucket "%s" created`, name)

			// Cost reporting requires a policy for the reporting job to put objects in the bucket
			if step.Payload == "\"create_cost_policy\"" {
				l.Log.Debugf(`Creating S3 bucket policy for "%s"`, name)

				payload := substituteInPayload(amazon.CostS3Policy, f, step.Substitutions)

				err := a.Client.AttachBucketPolicy(ctx, name, payload)
				if err != nil {
					return f, superkey.NewForgeError("s3_policy", name, fmt.Errorf("failed to attach bucket policy: %w", err), f)
				}

				l.Log.Infof(`S3 bucket policy attached to bucket "%s"`, name)
			}

		case "cost_report":
			payload := substituteInPayload(step.Payload, f, step.Substitutions)
			costReport := amazon.CostReport{}

			err := json.Unmarshal([]byte(payload), &costReport)
			if err != nil {
				return f, superkey.NewForgeError("cost_report", "", fmt.Errorf("failed to parse cost report payload: %w", err), f)
			}

			costReport.ReportName = fmt.Sprintf("%v-%v", costReport.ReportName, f.GUID)

			l.Log.Debugf(`Creating cost and usage report "%s"`, costReport.ReportName)

			err = a.Client.CreateCostAndUsageReport(ctx, &costReport)
			if err != nil {
				return f, superkey.NewForgeError("cost_report", costReport.ReportName, fmt.Errorf("failed to create cost report: %w", err), f)
			}

			f.StepsCompleted["cost_report"] = map[string]string{"output": costReport.ReportName}
			l.Log.Infof(`Cost and usage report "%s" created`, costReport.ReportName)

		case "policy":
			name := fmt.Sprintf("%v-policy-%v", getShortName(f.Request.ApplicationType), f.GUID)
			payload := substituteInPayload(step.Payload, f, step.Substitutions)

			l.Log.Debugf(`Creating policy "%s"`, name)

			arn, err := a.Client.CreatePolicy(ctx, name, payload)
			if err != nil {
				return f, superkey.NewForgeError("policy", name, fmt.Errorf("failed to create policy: %w", err), f)
			}

			f.StepsCompleted["policy"] = map[string]string{"output": *arn}
			l.Log.Infof(`Policy "%s" created`, name)

		case "role":
			name := fmt.Sprintf("%v-role-%v", getShortName(f.Request.ApplicationType), f.GUID)
			payload := substituteInPayload(step.Payload, f, step.Substitutions)

			l.Log.Debugf(`Creating role "%s"`, name)

			roleArn, err := a.Client.CreateRole(ctx, name, payload)
			if err != nil {
				return f, superkey.NewForgeError("role", name, fmt.Errorf("failed to create role: %w", err), f)
			}

			// Store the Role ARN since that is what we need for the Authentication object
			f.StepsCompleted["role"] = map[string]string{"output": name, "arn": *roleArn}
			l.Log.Infof(`Role "%s" created`, name)

		case "bind_role":
			roleName := f.StepsCompleted["role"]["output"]
			policyArn := f.StepsCompleted["policy"]["output"]

			l.Log.Debugf(`Binding role "%s" to policy "%s"`, roleName, policyArn)

			err := a.Client.BindPolicyToRole(ctx, policyArn, roleName)
			if err != nil {
				return f, superkey.NewForgeError("bind_role", roleName, fmt.Errorf("failed to bind policy to role: %w", err), f)
			}

			f.StepsCompleted["bind_role"] = map[string]string{}
			l.Log.Infof(`Bound role "%s" to policy "%s"`, roleName, policyArn)

		default:
			return f, superkey.NewForgeError(step.Name, "", fmt.Errorf("superkey step %q not implemented", step.Name), f)
		}
	}

	// Set the username to the role ARN since that is what is needed for this provider
	username := f.StepsCompleted["role"]["arn"]
	appType := path.Base(f.Request.ApplicationType)
	// Create the payload struct
	f.CreatePayload(&username, nil, &appType)

	return f, nil
}

// TearDown removes AWS resources in reverse order
func (a *AmazonProvider) TearDown(ctx context.Context, f *superkey.ForgedApplication) []error {
	errors := make([]error, 0)

	// Unbind the role first so we can cleanly delete the policy and role
	if f.StepsCompleted["bind_role"] != nil {
		policyArn := f.StepsCompleted["policy"]["output"]
		role := f.StepsCompleted["role"]["output"]

		err := a.Client.UnBindPolicyToRole(ctx, policyArn, role)
		if err != nil {
			errors = append(errors, fmt.Errorf(`failed to unbind policy "%s" from role "%s": %w`, policyArn, role, err))
		} else {
			l.Log.Infof(`Policy "%s" unbound from role "%s"`, policyArn, role)
		}
	}

	// Role/policy/reporting can be deleted independently
	if f.StepsCompleted["policy"] != nil {
		policyArn := f.StepsCompleted["policy"]["output"]

		err := a.Client.DestroyPolicy(ctx, policyArn)
		if err != nil {
			errors = append(errors, fmt.Errorf(`failed to destroy policy "%s": %w`, policyArn, err))
		} else {
			l.Log.Infof(`Policy "%s" destroyed`, policyArn)
		}
	}

	if f.StepsCompleted["role"] != nil {
		roleName := f.StepsCompleted["role"]["output"]

		err := a.Client.DestroyRole(ctx, roleName)
		if err != nil {
			errors = append(errors, fmt.Errorf(`failed to destroy role "%s": %w`, roleName, err))
		} else {
			l.Log.Infof(`Role "%s" destroyed`, roleName)
		}
	}

	if f.StepsCompleted["cost_report"] != nil {
		reportName := f.StepsCompleted["cost_report"]["output"]

		err := a.Client.DestroyCostAndUsageReport(ctx, reportName)
		if err != nil {
			errors = append(errors, fmt.Errorf(`failed to destroy cost and usage report "%s": %w`, reportName, err))
		} else {
			l.Log.Infof(`Cost and usage report "%s" destroyed`, reportName)
		}
	}

	// S3 bucket deleted last in case other things depend on it
	if f.StepsCompleted["s3"] != nil {
		bucket := f.StepsCompleted["s3"]["output"]

		err := a.Client.DestroyS3Bucket(ctx, bucket)
		if err != nil {
			errors = append(errors, fmt.Errorf(`failed to destroy S3 bucket "%s": %w`, bucket, err))
		} else {
			l.Log.Infof(`S3 bucket "%s" destroyed`, bucket)
		}
	}

	return errors
}

// Ensure AmazonProvider implements Provider
var _ superkey.Provider = (*AmazonProvider)(nil)
