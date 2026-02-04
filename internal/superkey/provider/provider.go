package provider

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/amazon"
)

// GetProvider returns the appropriate provider based on the provider name
func GetProvider(ctx context.Context, providerName string, superKeyAuth string) (superkey.Provider, error) {
	switch providerName {
	case "amazon":
		return NewAmazonProvider(ctx, superKeyAuth)
	default:
		return nil, fmt.Errorf("unsupported superkey provider: %s", providerName)
	}
}

// GetProviderWithClient returns a provider with an injected client (for testing)
func GetProviderWithClient(providerName string, client amazon.AmazonClient) (superkey.Provider, error) {
	switch providerName {
	case "amazon":
		return &AmazonProvider{Client: client}, nil
	default:
		return nil, fmt.Errorf("unsupported superkey provider: %s", providerName)
	}
}
