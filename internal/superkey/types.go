package superkey

import (
	"context"

	"github.com/RedHatInsights/sources-api-go/model"
)

// CreateRequest represents a request to create superkey resources
type CreateRequest struct {
	IdentityHeader  string            `json:"identity_header"`
	OrgIdHeader     string            `json:"org_id_header"`
	TenantID        string            `json:"tenant_id"`
	SourceID        string            `json:"source_id"`
	ApplicationID   string            `json:"application_id"`
	ApplicationType string            `json:"application_type"`
	SuperKey        string            `json:"super_key"`
	Provider        string            `json:"provider"`
	Extra           map[string]string `json:"extra"`
	SuperKeySteps   []Step            `json:"superkey_steps"`
}

// Step represents a step for SuperKey resource creation
type Step struct {
	Step          int               `json:"step"`
	Name          string            `json:"name"`
	Payload       string            `json:"payload"`
	Substitutions map[string]string `json:"substitutions"`
}

// DestroyRequest represents a teardown request for an application
// created through superkey
type DestroyRequest struct {
	TenantID       string                       `json:"tenant_id"`
	SuperKey       string                       `json:"super_key"`
	GUID           string                       `json:"guid"`
	Provider       string                       `json:"provider"`
	StepsCompleted map[string]map[string]string `json:"steps_completed"`
	SuperKeySteps  []Step                       `json:"superkey_steps"`
}

// App represents an application that can be posted to sources after being
// populated with superkey data
type App struct {
	SourceID    string                            `json:"source_id"`
	Extra       map[string]interface{}            `json:"extra"`
	AuthPayload model.AuthenticationCreateRequest `json:"authentication_payload"`
}

// ForgedApplication holds the output of a superkey create_application request
type ForgedApplication struct {
	Product        *App
	StepsCompleted map[string]map[string]string
	Request        *CreateRequest
	Client         Provider
	GUID           string
}

// Provider is the interface for all superkey providers
type Provider interface {
	ForgeApplication(ctx context.Context, createRequest *CreateRequest) (*ForgedApplication, error)
	TearDown(ctx context.Context, forgedApplication *ForgedApplication) []error
}

// Result provides structured outcome of superkey operations
type Result struct {
	Success        bool
	ForgedApp      *ForgedApplication
	Error          error
	TearDownErrors []error
}
