package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// CreateRole creates an IAM role with the given name and assume role policy document
func (a *Client) CreateRole(ctx context.Context, name, payload string) (*string, error) {
	iamRole, err := a.Iam.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &payload,
		RoleName:                 &name,
	})

	if err != nil {
		return nil, err
	}

	return iamRole.Role.Arn, nil
}

// DestroyRole deletes an IAM role by name
func (a *Client) DestroyRole(ctx context.Context, name string) error {
	_, err := a.Iam.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: &name,
	})

	return err
}

// CreatePolicy creates an IAM policy with the given name and policy document
func (a *Client) CreatePolicy(ctx context.Context, name, payload string) (*string, error) {
	out, err := a.Iam.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyDocument: &payload,
		PolicyName:     &name,
	})

	if err != nil {
		return nil, err
	}

	return out.Policy.Arn, nil
}

// DestroyPolicy deletes an IAM policy by ARN
func (a *Client) DestroyPolicy(ctx context.Context, arn string) error {
	_, err := a.Iam.DeletePolicy(ctx, &iam.DeletePolicyInput{
		PolicyArn: &arn,
	})

	return err
}

// BindPolicyToRole attaches a policy to a role
func (a *Client) BindPolicyToRole(ctx context.Context, policy, role string) error {
	_, err := a.Iam.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: &policy,
		RoleName:  &role,
	})

	return err
}

// UnBindPolicyToRole detaches a policy from a role
func (a *Client) UnBindPolicyToRole(ctx context.Context, policy, role string) error {
	_, err := a.Iam.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		PolicyArn: &policy,
		RoleName:  &role,
	})

	return err
}
