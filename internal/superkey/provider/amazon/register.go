package amazon

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
)

func init() {
	// Register the Amazon provider
	provider.RegisterProvider("amazon", NewAmazonProviderFromCredentials)
}

// NewAmazonProviderFromCredentials creates a new Amazon provider from credentials
func NewAmazonProviderFromCredentials(ctx context.Context, credentials provider.Credentials, steps []provider.Step) (provider.Provider, error) {
	stepNames := provider.GetStepNames(steps)
	client, err := NewClient(ctx, credentials.Username, credentials.Password, stepNames...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Amazon client: %w", err)
	}
	return NewAmazonProvider(client), nil
}
