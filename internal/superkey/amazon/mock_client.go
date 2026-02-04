package amazon

import (
	"context"
)

// MockClient is a mock implementation of AmazonClient for testing
type MockClient struct {
	CreateRoleFunc              func(ctx context.Context, name, payload string) (*string, error)
	DestroyRoleFunc             func(ctx context.Context, name string) error
	CreatePolicyFunc            func(ctx context.Context, name, payload string) (*string, error)
	DestroyPolicyFunc           func(ctx context.Context, arn string) error
	BindPolicyToRoleFunc        func(ctx context.Context, policy, role string) error
	UnBindPolicyToRoleFunc      func(ctx context.Context, policy, role string) error
	CreateS3BucketFunc          func(ctx context.Context, name string) error
	DestroyS3BucketFunc         func(ctx context.Context, name string) error
	AttachBucketPolicyFunc      func(ctx context.Context, bucket, policy string) error
	CreateCostAndUsageReportFunc func(ctx context.Context, report *CostReport) error
	DestroyCostAndUsageReportFunc func(ctx context.Context, name string) error
}

func (m *MockClient) CreateRole(ctx context.Context, name, payload string) (*string, error) {
	if m.CreateRoleFunc != nil {
		return m.CreateRoleFunc(ctx, name, payload)
	}
	arn := "arn:aws:iam::123456789012:role/" + name
	return &arn, nil
}

func (m *MockClient) DestroyRole(ctx context.Context, name string) error {
	if m.DestroyRoleFunc != nil {
		return m.DestroyRoleFunc(ctx, name)
	}
	return nil
}

func (m *MockClient) CreatePolicy(ctx context.Context, name, payload string) (*string, error) {
	if m.CreatePolicyFunc != nil {
		return m.CreatePolicyFunc(ctx, name, payload)
	}
	arn := "arn:aws:iam::123456789012:policy/" + name
	return &arn, nil
}

func (m *MockClient) DestroyPolicy(ctx context.Context, arn string) error {
	if m.DestroyPolicyFunc != nil {
		return m.DestroyPolicyFunc(ctx, arn)
	}
	return nil
}

func (m *MockClient) BindPolicyToRole(ctx context.Context, policy, role string) error {
	if m.BindPolicyToRoleFunc != nil {
		return m.BindPolicyToRoleFunc(ctx, policy, role)
	}
	return nil
}

func (m *MockClient) UnBindPolicyToRole(ctx context.Context, policy, role string) error {
	if m.UnBindPolicyToRoleFunc != nil {
		return m.UnBindPolicyToRoleFunc(ctx, policy, role)
	}
	return nil
}

func (m *MockClient) CreateS3Bucket(ctx context.Context, name string) error {
	if m.CreateS3BucketFunc != nil {
		return m.CreateS3BucketFunc(ctx, name)
	}
	return nil
}

func (m *MockClient) DestroyS3Bucket(ctx context.Context, name string) error {
	if m.DestroyS3BucketFunc != nil {
		return m.DestroyS3BucketFunc(ctx, name)
	}
	return nil
}

func (m *MockClient) AttachBucketPolicy(ctx context.Context, bucket, policy string) error {
	if m.AttachBucketPolicyFunc != nil {
		return m.AttachBucketPolicyFunc(ctx, bucket, policy)
	}
	return nil
}

func (m *MockClient) CreateCostAndUsageReport(ctx context.Context, report *CostReport) error {
	if m.CreateCostAndUsageReportFunc != nil {
		return m.CreateCostAndUsageReportFunc(ctx, report)
	}
	return nil
}

func (m *MockClient) DestroyCostAndUsageReport(ctx context.Context, name string) error {
	if m.DestroyCostAndUsageReportFunc != nil {
		return m.DestroyCostAndUsageReportFunc(ctx, name)
	}
	return nil
}

// Ensure MockClient implements AmazonClient
var _ AmazonClient = (*MockClient)(nil)
