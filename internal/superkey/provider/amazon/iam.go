package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// CreateRole creates an IAM role with the given name and policy document
func (c *Client) CreateRole(ctx context.Context, name, policyDocument string) (*string, error) {
	output, err := c.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &policyDocument,
		RoleName:                 &name,
	})
	if err != nil {
		return nil, err
	}
	return output.Role.Arn, nil
}

// DestroyRole deletes an IAM role
func (c *Client) DestroyRole(ctx context.Context, name string) error {
	_, err := c.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: &name,
	})
	return err
}

// CreatePolicy creates an IAM policy with the given name and policy document
func (c *Client) CreatePolicy(ctx context.Context, name, policyDocument string) (*string, error) {
	output, err := c.IAM.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyDocument: &policyDocument,
		PolicyName:     &name,
	})
	if err != nil {
		return nil, err
	}
	return output.Policy.Arn, nil
}

// DestroyPolicy deletes an IAM policy by ARN
func (c *Client) DestroyPolicy(ctx context.Context, arn string) error {
	_, err := c.IAM.DeletePolicy(ctx, &iam.DeletePolicyInput{
		PolicyArn: &arn,
	})
	return err
}

// BindPolicyToRole attaches a policy to a role
func (c *Client) BindPolicyToRole(ctx context.Context, policyArn, roleName string) error {
	_, err := c.IAM.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: &policyArn,
		RoleName:  &roleName,
	})
	return err
}

// UnBindPolicyToRole detaches a policy from a role
func (c *Client) UnBindPolicyToRole(ctx context.Context, policyArn, roleName string) error {
	_, err := c.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		PolicyArn: &policyArn,
		RoleName:  &roleName,
	})
	return err
}
