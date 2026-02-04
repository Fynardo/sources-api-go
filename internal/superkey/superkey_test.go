package superkey

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/sirupsen/logrus"

	l "github.com/RedHatInsights/sources-api-go/logger"
)

func init() {
	// Initialize logger for tests
	l.Log = &logrus.Logger{
		Out:       os.Stdout,
		Level:     logrus.DebugLevel,
		Formatter: &logrus.TextFormatter{},
	}
}

// MockProvider is a mock implementation of Provider for testing
type MockProvider struct {
	ForgeFunc    func(ctx context.Context, req *CreateRequest) (*ForgedApplication, error)
	TearDownFunc func(ctx context.Context, app *ForgedApplication) []error
}

func (m *MockProvider) ForgeApplication(ctx context.Context, req *CreateRequest) (*ForgedApplication, error) {
	if m.ForgeFunc != nil {
		return m.ForgeFunc(ctx, req)
	}
	return &ForgedApplication{
		StepsCompleted: make(map[string]map[string]string),
		Request:        req,
		GUID:           "test-guid",
	}, nil
}

func (m *MockProvider) TearDown(ctx context.Context, app *ForgedApplication) []error {
	if m.TearDownFunc != nil {
		return m.TearDownFunc(ctx, app)
	}
	return nil
}

func TestProcessCreate_Success(t *testing.T) {
	ctx := context.Background()
	req := &CreateRequest{
		TenantID:        "12345",
		SourceID:        "1",
		ApplicationID:   "1",
		ApplicationType: "/insights/platform/cost-management",
		Provider:        "amazon",
		Extra:           map[string]string{"account": "123456789012"},
		SuperKeySteps: []Step{
			{Step: 1, Name: "policy", Payload: "{}"},
			{Step: 2, Name: "role", Payload: "{}"},
		},
	}

	mockProvider := &MockProvider{
		ForgeFunc: func(ctx context.Context, req *CreateRequest) (*ForgedApplication, error) {
			return &ForgedApplication{
				StepsCompleted: map[string]map[string]string{
					"policy": {"output": "arn:aws:iam::123456789012:policy/test-policy"},
					"role":   {"output": "test-role", "arn": "arn:aws:iam::123456789012:role/test-role"},
				},
				Request: req,
				GUID:    "test-guid-12345678",
				Product: &App{
					SourceID: req.SourceID,
					Extra:    map[string]interface{}{"_superkey": map[string]interface{}{"guid": "test-guid-12345678"}},
				},
			}, nil
		},
	}

	result := ProcessCreate(ctx, req, mockProvider)

	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Error)
	}

	if result.ForgedApp == nil {
		t.Error("expected forged app, got nil")
	}

	if result.ForgedApp.GUID != "test-guid-12345678" {
		t.Errorf("expected GUID 'test-guid-12345678', got '%s'", result.ForgedApp.GUID)
	}
}

func TestProcessCreate_Failure_WithRollback(t *testing.T) {
	ctx := context.Background()
	req := &CreateRequest{
		TenantID:        "12345",
		SourceID:        "1",
		ApplicationID:   "1",
		ApplicationType: "/insights/platform/cost-management",
		Provider:        "amazon",
		SuperKeySteps: []Step{
			{Step: 1, Name: "policy", Payload: "{}"},
			{Step: 2, Name: "role", Payload: "{}"},
		},
	}

	teardownCalled := false

	mockProvider := &MockProvider{
		ForgeFunc: func(ctx context.Context, req *CreateRequest) (*ForgedApplication, error) {
			// Return partial state with error
			return &ForgedApplication{
				StepsCompleted: map[string]map[string]string{
					"policy": {"output": "arn:aws:iam::123456789012:policy/test-policy"},
				},
				Request: req,
				GUID:    "test-guid",
			}, errors.New("failed to create role")
		},
		TearDownFunc: func(ctx context.Context, app *ForgedApplication) []error {
			teardownCalled = true
			return nil
		},
	}

	result := ProcessCreate(ctx, req, mockProvider)

	if result.Success {
		t.Error("expected failure, got success")
	}

	if result.Error == nil {
		t.Error("expected error, got nil")
	}

	if !teardownCalled {
		t.Error("expected teardown to be called for rollback")
	}
}

func TestProcessDestroy_Success(t *testing.T) {
	ctx := context.Background()
	req := &DestroyRequest{
		TenantID: "12345",
		GUID:     "test-guid-12345678",
		Provider: "amazon",
		StepsCompleted: map[string]map[string]string{
			"policy":    {"output": "arn:aws:iam::123456789012:policy/test-policy"},
			"role":      {"output": "test-role", "arn": "arn:aws:iam::123456789012:role/test-role"},
			"bind_role": {},
		},
	}

	teardownCalled := false

	mockProvider := &MockProvider{
		TearDownFunc: func(ctx context.Context, app *ForgedApplication) []error {
			teardownCalled = true
			return nil
		},
	}

	result := ProcessDestroy(ctx, req, mockProvider)

	if !result.Success {
		t.Error("expected success, got failure")
	}

	if !teardownCalled {
		t.Error("expected teardown to be called")
	}
}

func TestProcessDestroy_WithErrors(t *testing.T) {
	ctx := context.Background()
	req := &DestroyRequest{
		TenantID: "12345",
		GUID:     "test-guid-12345678",
		Provider: "amazon",
		StepsCompleted: map[string]map[string]string{
			"policy": {"output": "arn:aws:iam::123456789012:policy/test-policy"},
			"role":   {"output": "test-role"},
		},
	}

	mockProvider := &MockProvider{
		TearDownFunc: func(ctx context.Context, app *ForgedApplication) []error {
			return []error{
				errors.New("failed to delete policy"),
				errors.New("failed to delete role"),
			}
		},
	}

	result := ProcessDestroy(ctx, req, mockProvider)

	if result.Success {
		t.Error("expected failure due to teardown errors")
	}

	if len(result.TearDownErrors) != 2 {
		t.Errorf("expected 2 teardown errors, got %d", len(result.TearDownErrors))
	}
}

func TestReconstructForgedApplication(t *testing.T) {
	req := &DestroyRequest{
		TenantID:  "12345",
		SuperKey:  "auth-123",
		GUID:      "test-guid",
		Provider:  "amazon",
		StepsCompleted: map[string]map[string]string{
			"s3":     {"output": "test-bucket"},
			"policy": {"output": "arn:aws:iam::123456789012:policy/test-policy"},
		},
		SuperKeySteps: []Step{
			{Step: 1, Name: "s3"},
			{Step: 2, Name: "policy"},
		},
	}

	forgedApp := ReconstructForgedApplication(req)

	if forgedApp.GUID != "test-guid" {
		t.Errorf("expected GUID 'test-guid', got '%s'", forgedApp.GUID)
	}

	if forgedApp.Request.Provider != "amazon" {
		t.Errorf("expected provider 'amazon', got '%s'", forgedApp.Request.Provider)
	}

	if len(forgedApp.StepsCompleted) != 2 {
		t.Errorf("expected 2 steps completed, got %d", len(forgedApp.StepsCompleted))
	}
}
