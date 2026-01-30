package provider

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/sirupsen/logrus"

	l "github.com/RedHatInsights/sources-api-go/logger"

	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/amazon"
)

func init() {
	// Initialize logger for tests
	l.Log = &logrus.Logger{
		Out:       os.Stdout,
		Level:     logrus.DebugLevel,
		Formatter: &logrus.TextFormatter{},
	}
}

func TestForgeApplication_Success(t *testing.T) {
	ctx := context.Background()

	mockClient := &amazon.MockClient{
		CreatePolicyFunc: func(ctx context.Context, name, payload string) (*string, error) {
			arn := "arn:aws:iam::123456789012:policy/" + name
			return &arn, nil
		},
		CreateRoleFunc: func(ctx context.Context, name, payload string) (*string, error) {
			arn := "arn:aws:iam::123456789012:role/" + name
			return &arn, nil
		},
		BindPolicyToRoleFunc: func(ctx context.Context, policy, role string) error {
			return nil
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	req := &superkey.CreateRequest{
		TenantID:        "12345",
		SourceID:        "1",
		ApplicationID:   "1",
		ApplicationType: "/insights/platform/cost-management",
		Provider:        "amazon",
		Extra: map[string]string{
			"account":     "123456789012",
			"result_type": "arn",
			"external_id": "test-external-id",
		},
		SuperKeySteps: []superkey.Step{
			{Step: 1, Name: "policy", Payload: `{"Version": "2012-10-17"}`},
			{Step: 2, Name: "role", Payload: `{"Version": "2012-10-17"}`},
			{Step: 3, Name: "bind_role"},
		},
	}

	forgedApp, err := provider.ForgeApplication(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if forgedApp == nil {
		t.Fatal("expected forged app, got nil")
	}

	if forgedApp.GUID == "" {
		t.Error("expected GUID to be set")
	}

	if _, ok := forgedApp.StepsCompleted["policy"]; !ok {
		t.Error("expected policy step to be completed")
	}

	if _, ok := forgedApp.StepsCompleted["role"]; !ok {
		t.Error("expected role step to be completed")
	}

	if _, ok := forgedApp.StepsCompleted["bind_role"]; !ok {
		t.Error("expected bind_role step to be completed")
	}
}

func TestForgeApplication_S3Step(t *testing.T) {
	ctx := context.Background()

	s3Created := false
	mockClient := &amazon.MockClient{
		CreateS3BucketFunc: func(ctx context.Context, name string) error {
			s3Created = true
			return nil
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	req := &superkey.CreateRequest{
		TenantID:        "12345",
		SourceID:        "1",
		ApplicationID:   "1",
		ApplicationType: "/insights/platform/cost-management",
		Provider:        "amazon",
		Extra:           map[string]string{"account": "123456789012"},
		SuperKeySteps: []superkey.Step{
			{Step: 1, Name: "s3", Payload: ""},
		},
	}

	forgedApp, err := provider.ForgeApplication(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s3Created {
		t.Error("expected S3 bucket to be created")
	}

	if _, ok := forgedApp.StepsCompleted["s3"]; !ok {
		t.Error("expected s3 step to be completed")
	}
}

func TestForgeApplication_Failure_RollbackState(t *testing.T) {
	ctx := context.Background()

	mockClient := &amazon.MockClient{
		CreatePolicyFunc: func(ctx context.Context, name, payload string) (*string, error) {
			arn := "arn:aws:iam::123456789012:policy/" + name
			return &arn, nil
		},
		CreateRoleFunc: func(ctx context.Context, name, payload string) (*string, error) {
			return nil, errors.New("failed to create role")
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	req := &superkey.CreateRequest{
		TenantID:        "12345",
		SourceID:        "1",
		ApplicationID:   "1",
		ApplicationType: "/insights/platform/cost-management",
		Provider:        "amazon",
		Extra:           map[string]string{"account": "123456789012"},
		SuperKeySteps: []superkey.Step{
			{Step: 1, Name: "policy", Payload: `{}`},
			{Step: 2, Name: "role", Payload: `{}`},
		},
	}

	forgedApp, err := provider.ForgeApplication(ctx, req)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should have partial state for rollback
	if forgedApp == nil {
		t.Fatal("expected partial forged app for rollback, got nil")
	}

	if _, ok := forgedApp.StepsCompleted["policy"]; !ok {
		t.Error("expected policy step to be completed before failure")
	}

	if _, ok := forgedApp.StepsCompleted["role"]; ok {
		t.Error("role step should NOT be completed since it failed")
	}
}

func TestTearDown_Success(t *testing.T) {
	ctx := context.Background()

	unbindCalled := false
	policyDeleted := false
	roleDeleted := false

	mockClient := &amazon.MockClient{
		UnBindPolicyToRoleFunc: func(ctx context.Context, policy, role string) error {
			unbindCalled = true
			return nil
		},
		DestroyPolicyFunc: func(ctx context.Context, arn string) error {
			policyDeleted = true
			return nil
		},
		DestroyRoleFunc: func(ctx context.Context, name string) error {
			roleDeleted = true
			return nil
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	forgedApp := &superkey.ForgedApplication{
		StepsCompleted: map[string]map[string]string{
			"policy":    {"output": "arn:aws:iam::123456789012:policy/test-policy"},
			"role":      {"output": "test-role", "arn": "arn:aws:iam::123456789012:role/test-role"},
			"bind_role": {},
		},
		Request: &superkey.CreateRequest{},
		GUID:    "test-guid",
	}

	errs := provider.TearDown(ctx, forgedApp)

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	if !unbindCalled {
		t.Error("expected unbind to be called")
	}

	if !policyDeleted {
		t.Error("expected policy to be deleted")
	}

	if !roleDeleted {
		t.Error("expected role to be deleted")
	}
}

func TestTearDown_PartialFailure(t *testing.T) {
	ctx := context.Background()

	mockClient := &amazon.MockClient{
		UnBindPolicyToRoleFunc: func(ctx context.Context, policy, role string) error {
			return nil
		},
		DestroyPolicyFunc: func(ctx context.Context, arn string) error {
			return errors.New("failed to delete policy")
		},
		DestroyRoleFunc: func(ctx context.Context, name string) error {
			return nil
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	forgedApp := &superkey.ForgedApplication{
		StepsCompleted: map[string]map[string]string{
			"policy":    {"output": "arn:aws:iam::123456789012:policy/test-policy"},
			"role":      {"output": "test-role"},
			"bind_role": {},
		},
		Request: &superkey.CreateRequest{},
		GUID:    "test-guid",
	}

	errs := provider.TearDown(ctx, forgedApp)

	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestTearDown_WithS3(t *testing.T) {
	ctx := context.Background()

	s3Deleted := false

	mockClient := &amazon.MockClient{
		DestroyS3BucketFunc: func(ctx context.Context, name string) error {
			s3Deleted = true
			return nil
		},
	}

	provider := &AmazonProvider{Client: mockClient}

	forgedApp := &superkey.ForgedApplication{
		StepsCompleted: map[string]map[string]string{
			"s3": {"output": "test-bucket"},
		},
		Request: &superkey.CreateRequest{},
		GUID:    "test-guid",
	}

	errs := provider.TearDown(ctx, forgedApp)

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	if !s3Deleted {
		t.Error("expected S3 bucket to be deleted")
	}
}
