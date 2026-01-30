package provider

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/dao"
)

// Credentials holds the authentication credentials for a provider
type Credentials struct {
	Username string
	Password string
}

// ProviderFactory is a function that creates a Provider
type ProviderFactory func(ctx context.Context, credentials Credentials, steps []Step) (Provider, error)

// Registry holds provider factories
var providerRegistry = make(map[string]ProviderFactory)

// RegisterProvider registers a provider factory
func RegisterProvider(name string, factory ProviderFactory) {
	providerRegistry[name] = factory
}

// NewProvider creates a new provider instance based on the provider name and credentials
func NewProvider(ctx context.Context, providerName string, credentials Credentials, steps []Step) (Provider, error) {
	factory, ok := providerRegistry[providerName]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
	return factory(ctx, credentials, steps)
}

// GetProviderForRequest creates a provider instance from a CreateRequest
// It fetches the necessary authentication credentials from the database
func GetProviderForRequest(ctx context.Context, request *CreateRequest, authDao dao.AuthenticationDao) (Provider, error) {
	// Fetch the superkey authentication (contains AWS access key and secret key)
	auth, err := authDao.GetById(fmt.Sprintf("%d", request.SuperKey))
	if err != nil {
		return nil, fmt.Errorf("error fetching authentication %d: %w", request.SuperKey, err)
	}

	// Validate credentials
	username := ""
	password := ""
	if auth.Username != nil {
		username = *auth.Username
	}
	if auth.Password != nil {
		password = *auth.Password
	}

	if username == "" || password == "" {
		return nil, fmt.Errorf("missing username or password from authentication ID %d", request.SuperKey)
	}

	credentials := Credentials{
		Username: username,
		Password: password,
	}

	return NewProvider(ctx, request.Provider, credentials, request.SuperKeySteps)
}

// getStepNames extracts step names from the list of steps
func getStepNames(steps []Step) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name)
	}
	return names
}

// ReconstructFromDestroyRequest creates a Result from a DestroyRequest
func ReconstructFromDestroyRequest(request *DestroyRequest) *Result {
	return &Result{
		StepsCompleted: request.StepsCompleted,
		Request: &CreateRequest{
			TenantID:      request.TenantID,
			SuperKey:      request.SuperKey,
			Provider:      request.Provider,
			SuperKeySteps: request.SuperKeySteps,
		},
		GUID: request.GUID,
	}
}

// Helper function to get step names (exported for use by providers)
func GetStepNames(steps []Step) []string {
	return getStepNames(steps)
}
