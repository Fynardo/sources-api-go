package provider

import (
	"context"
)

// CreateRequest represents a request for superkey resource creation
// Defined here to avoid import cycles
type CreateRequest struct {
	TenantID        int64
	SourceID        int64
	ApplicationID   int64
	ApplicationType string
	SuperKey        int64
	Provider        string
	Extra           map[string]string
	SuperKeySteps   []Step
}

// DestroyRequest represents a request for superkey resource destruction
type DestroyRequest struct {
	TenantID       int64
	SuperKey       int64
	GUID           string
	Provider       string
	StepsCompleted map[string]map[string]string
	SuperKeySteps  []Step
}

// Step represents a step in the superkey process
type Step struct {
	Step          int
	Name          string
	Payload       string
	Substitutions map[string]string
}

// Result holds the output of superkey resource creation
type Result struct {
	StepsCompleted map[string]map[string]string
	Request        *CreateRequest
	GUID           string
	AuthType       string
	RoleARN        string
}

// MarkCompleted marks a step as completed
func (r *Result) MarkCompleted(name string, data map[string]string) {
	if r.StepsCompleted == nil {
		r.StepsCompleted = make(map[string]map[string]string)
	}
	r.StepsCompleted[name] = data
}

// GetRoleARN returns the ARN of the created role
func (r *Result) GetRoleARN() string {
	if r.StepsCompleted["role"] != nil {
		if arn, ok := r.StepsCompleted["role"]["arn"]; ok {
			return arn
		}
	}
	return ""
}

// Provider is the interface for all superkey providers
type Provider interface {
	ForgeApplication(ctx context.Context, request *CreateRequest) (*Result, error)
	TearDown(ctx context.Context, result *Result) []error
}
