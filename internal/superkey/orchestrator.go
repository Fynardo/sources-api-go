package superkey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RedHatInsights/sources-api-go/dao"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
	_ "github.com/RedHatInsights/sources-api-go/internal/superkey/provider/amazon" // Register amazon provider
	l "github.com/RedHatInsights/sources-api-go/logger"
	m "github.com/RedHatInsights/sources-api-go/model"
	"github.com/sirupsen/logrus"
)

const (
	// Maximum number of retries for transient errors
	maxRetries = 3
	// Initial backoff duration
	baseBackoff = 2 * time.Second
	// IAM eventual consistency delay
	iamWaitTime = 7 * time.Second
)

// Orchestrator manages the lifecycle of superkey resources
type Orchestrator struct {
	tenantID int64
}

// NewOrchestrator creates a new Orchestrator instance
func NewOrchestrator(tenantID int64) *Orchestrator {
	return &Orchestrator{
		tenantID: tenantID,
	}
}

// CreateResources orchestrates the creation of cloud resources for a superkey application
// It handles:
// - Creating AWS resources via the provider
// - Retry logic for transient failures
// - Database updates (application extra, superkey_data, availability_status)
// - Authentication record creation
func (o *Orchestrator) CreateResources(ctx context.Context, request *provider.CreateRequest) (*provider.Result, error) {
	logger := l.Log.WithFields(logrus.Fields{
		"tenant_id":        request.TenantID,
		"application_id":   request.ApplicationID,
		"application_type": request.ApplicationType,
		"provider":         request.Provider,
	})

	logger.Info("Starting superkey resource creation")

	// Get the authentication DAO to fetch credentials
	authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &request.TenantID})

	// Get the provider instance with credentials
	prov, err := provider.GetProviderForRequest(ctx, request, authDao)
	if err != nil {
		logger.WithError(err).Error("Failed to get provider")
		return nil, provider.WrapError("provider", "Failed to initialize cloud provider", err)
	}

	// Attempt resource creation with retry logic
	var result *provider.Result
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Infof("Attempt %d/%d to create resources", attempt, maxRetries)

		result, lastErr = prov.ForgeApplication(ctx, request)
		if lastErr == nil {
			logger.Info("Successfully created all resources")
			break
		}

		// Check if the error is retryable
		if !provider.IsRetryable(lastErr) {
			logger.WithError(lastErr).Error("Non-retryable error occurred")
			break
		}

		logger.WithError(lastErr).Warnf("Retryable error on attempt %d", attempt)

		// If this isn't the last attempt, wait before retrying
		if attempt < maxRetries {
			backoff := baseBackoff * time.Duration(1<<(attempt-1)) // Exponential backoff
			logger.Infof("Waiting %v before retry", backoff)
			time.Sleep(backoff)
		}
	}

	// If creation failed, return the error
	if lastErr != nil {
		logger.WithError(lastErr).Error("Failed to create resources after all retries")
		return result, lastErr
	}

	// Wait for IAM eventual consistency
	logger.Debugf("Waiting %v for IAM eventual consistency", iamWaitTime)
	time.Sleep(iamWaitTime)

	logger.Info("Superkey resource creation completed successfully")
	return result, nil
}

// DestroyResources orchestrates the destruction of cloud resources for a superkey application
// It handles:
// - Destroying AWS resources via the provider
// - Collecting and logging errors (non-fatal)
func (o *Orchestrator) DestroyResources(ctx context.Context, request *provider.DestroyRequest) ([]error, error) {
	logger := l.Log.WithFields(logrus.Fields{
		"tenant_id": request.TenantID,
		"provider":  request.Provider,
		"guid":      request.GUID,
	})

	logger.Info("Starting superkey resource destruction")

	// Get the authentication DAO to fetch credentials
	authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &request.TenantID})

	// Reconstruct the request for provider instantiation
	createRequest := &provider.CreateRequest{
		TenantID:      request.TenantID,
		SuperKey:      request.SuperKey,
		Provider:      request.Provider,
		SuperKeySteps: request.SuperKeySteps,
	}

	// Get the provider instance
	prov, err := provider.GetProviderForRequest(ctx, createRequest, authDao)
	if err != nil {
		logger.WithError(err).Error("Failed to get provider for teardown")
		return nil, fmt.Errorf("failed to initialize cloud provider: %w", err)
	}

	// Reconstruct the result from the destroy request
	result := provider.ReconstructFromDestroyRequest(request)

	// Attempt to tear down resources
	errors := prov.TearDown(ctx, result)

	if len(errors) > 0 {
		logger.Warnf("Encountered %d errors during resource destruction", len(errors))
		for i, err := range errors {
			logger.WithError(err).Warnf("Destruction error %d", i+1)
		}
	} else {
		logger.Info("All resources destroyed successfully")
	}

	return errors, nil
}

// UpdateApplicationWithResult updates the application in the database with the superkey result
// This includes setting extra, superkey_data, and availability_status fields
func (o *Orchestrator) UpdateApplicationWithResult(ctx context.Context, applicationID int64, result *provider.Result, authType string) error {
	logger := l.Log.WithFields(logrus.Fields{
		"tenant_id":      result.Request.TenantID,
		"application_id": applicationID,
	})

	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &result.Request.TenantID})

	// Fetch the application
	app, err := appDao.GetById(&applicationID)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch application")
		return fmt.Errorf("failed to fetch application: %w", err)
	}

	// Update superkey_data field
	superkeyData := map[string]interface{}{
		"steps":    result.StepsCompleted,
		"guid":     result.GUID,
		"provider": result.Request.Provider,
	}
	skData, _ := json.Marshal(superkeyData)
	app.SuperkeyData = skData

	// Update extra field
	extra := map[string]interface{}{
		"_superkey": superkeyData,
	}

	// Add S3 bucket if it was created
	if result.StepsCompleted["s3"] != nil {
		extra["bucket"] = result.StepsCompleted["s3"]["output"]
	}

	extraData, _ := json.Marshal(extra)
	app.Extra = extraData

	// Mark as available
	app.AvailabilityStatus = m.Available
	app.AvailabilityStatusError = ""

	// Update the application
	err = appDao.Update(app)
	if err != nil {
		logger.WithError(err).Error("Failed to update application")
		return fmt.Errorf("failed to update application: %w", err)
	}

	logger.Info("Application updated successfully with superkey data")
	return nil
}

// UpdateApplicationOnError updates the application to mark it as unavailable on error
func (o *Orchestrator) UpdateApplicationOnError(ctx context.Context, applicationID int64, tenantID int64, err error) error {
	logger := l.Log.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"application_id": applicationID,
	})

	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &tenantID})

	// Fetch the application
	app, fetchErr := appDao.GetById(&applicationID)
	if fetchErr != nil {
		logger.WithError(fetchErr).Error("Failed to fetch application for error update")
		return fmt.Errorf("failed to fetch application: %w", fetchErr)
	}

	// Mark as unavailable
	app.AvailabilityStatus = m.Unavailable

	// Set error message
	var skErr *provider.SuperkeyError
	var ok bool
	if skErr, ok = err.(*provider.SuperkeyError); ok {
		app.AvailabilityStatusError = skErr.ToStatusError()
	} else {
		app.AvailabilityStatusError = err.Error()
	}

	// Update the application
	updateErr := appDao.Update(app)
	if updateErr != nil {
		logger.WithError(updateErr).Error("Failed to update application with error status")
		return fmt.Errorf("failed to update application: %w", updateErr)
	}

	logger.Info("Application marked as unavailable")
	return nil
}

// CreateAuthentication creates an authentication record for the superkey application
func (o *Orchestrator) CreateAuthentication(ctx context.Context, applicationID int64, tenantID int64, result *provider.Result) error {
	logger := l.Log.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"application_id": applicationID,
	})

	authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &tenantID})

	// Get the role ARN from the result
	roleARN := result.GetRoleARN()
	if roleARN == "" {
		return fmt.Errorf("no role ARN found in superkey result")
	}

	// Create authentication record
	auth := &m.Authentication{
		Username:     &roleARN,
		AuthType:     result.AuthType,
		ResourceID:   applicationID,
		ResourceType: "Application",
	}

	// Add external_id to extra if present
	if externalID, ok := result.Request.Extra["external_id"]; ok {
		auth.Extra = map[string]interface{}{
			"external_id": externalID,
		}
	}

	err := authDao.Create(auth)
	if err != nil {
		logger.WithError(err).Error("Failed to create authentication")
		return fmt.Errorf("failed to create authentication: %w", err)
	}

	logger.Infof("Authentication created with username: %s", roleARN)
	return nil
}
