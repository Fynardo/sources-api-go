package superkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/RedHatInsights/sources-api-go/config"
	"github.com/RedHatInsights/sources-api-go/dao"
	l "github.com/RedHatInsights/sources-api-go/logger"
	m "github.com/RedHatInsights/sources-api-go/model"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

const DefaultAWSWaitTime = 7

// ProcessCreate handles the complete superkey application creation flow
func ProcessCreate(ctx context.Context, req *CreateRequest, provider Provider) *Result {
	result := &Result{Success: false}

	// Create the resources via the provider
	forgedApp, err := provider.ForgeApplication(ctx, req)
	if err != nil {
		result.Error = err
		result.ForgedApp = forgedApp

		// Perform rollback if we have partial state
		if forgedApp != nil && len(forgedApp.StepsCompleted) > 0 {
			l.Log.WithFields(logrus.Fields{
				"application_id":  req.ApplicationID,
				"steps_completed": len(forgedApp.StepsCompleted),
			}).Info("Performing rollback of partially created resources")

			teardownErrors := provider.TearDown(ctx, forgedApp)
			if len(teardownErrors) > 0 {
				result.TearDownErrors = teardownErrors
				l.Log.WithFields(logrus.Fields{
					"application_id":  req.ApplicationID,
					"teardown_errors": len(teardownErrors),
				}).Warn("Errors during rollback")
			}
		}

		return result
	}

	result.ForgedApp = forgedApp
	result.Success = true

	return result
}

// ProcessDestroy handles the complete superkey application teardown flow
func ProcessDestroy(ctx context.Context, req *DestroyRequest, provider Provider) *Result {
	result := &Result{Success: false}

	// Reconstruct the forged application from the destroy request
	forgedApp := ReconstructForgedApplication(req)
	forgedApp.Client = provider
	result.ForgedApp = forgedApp

	// Tear down the resources
	errors := provider.TearDown(ctx, forgedApp)
	if len(errors) > 0 {
		result.TearDownErrors = errors
		l.Log.WithFields(logrus.Fields{
			"guid":            req.GUID,
			"teardown_errors": len(errors),
		}).Warn("Errors during teardown")
	} else {
		result.Success = true
	}

	return result
}

// PersistForgedApplication stores the forged application data in the database
func PersistForgedApplication(ctx context.Context, forgedApp *ForgedApplication) error {
	if forgedApp == nil || forgedApp.Product == nil {
		return fmt.Errorf("forged application or product is nil")
	}

	// Wait for IAM to propagate (prevents race condition)
	waitTime := getAWSWaitTime()
	l.Log.WithField("wait_seconds", waitTime).Debug("Sleeping to prevent IAM race condition")
	time.Sleep(time.Duration(waitTime) * time.Second)

	appID, err := strconv.ParseInt(forgedApp.Request.ApplicationID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid application ID: %w", err)
	}

	tenantID, err := getTenantIDFromExternal(forgedApp.Request.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Convert extra to JSON
	extraJSON, err := json.Marshal(forgedApp.Product.Extra)
	if err != nil {
		return fmt.Errorf("failed to marshal extra data: %w", err)
	}

	// Update the application with superkey data
	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &tenantID})
	err = appDao.Update(&m.Application{
		ID:    appID,
		Extra: datatypes.JSON(extraJSON),
	})
	if err != nil {
		return fmt.Errorf("failed to update application with superkey data: %w", err)
	}

	l.Log.WithField("application_id", appID).Info("Superkey data stored in application")

	// Create the authentication
	authReq := forgedApp.GetAuthenticationCreateRequest()
	if authReq != nil {
		authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &tenantID})

		auth := &m.Authentication{
			AuthType:     authReq.AuthType,
			Username:     authReq.Username,
			ResourceType: authReq.ResourceType,
			ResourceID:   appID,
		}

		if authReq.Extra != nil {
			if externalID, ok := authReq.Extra["external_id"].(string); ok {
				extraDbJSON, err := json.Marshal(map[string]interface{}{"external_id": externalID})
				if err == nil {
					auth.ExtraDb = datatypes.JSON(extraDbJSON)
				}
			}
		}

		err = authDao.Create(auth)
		if err != nil {
			return fmt.Errorf("failed to create authentication: %w", err)
		}

		l.Log.WithFields(logrus.Fields{
			"application_id": appID,
			"auth_type":      authReq.AuthType,
		}).Info("Authentication created for superkey application")

		// Create the application_authentication link
		appAuthDao := dao.GetApplicationAuthenticationDao(&dao.RequestParams{TenantID: &tenantID})
		err = appAuthDao.Create(&m.ApplicationAuthentication{
			ApplicationID:    appID,
			AuthenticationID: auth.DbID,
		})
		if err != nil {
			return fmt.Errorf("failed to create application authentication link: %w", err)
		}

		l.Log.WithField("application_id", appID).Info("Application authentication link created")
	}

	// Log completion
	sourceID, err := strconv.ParseInt(forgedApp.Request.SourceID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}

	l.Log.WithFields(logrus.Fields{
		"source_id":      sourceID,
		"application_id": appID,
	}).Info("Superkey application creation completed, availability check should be triggered")

	return nil
}

// MarkSourceUnavailable marks the application and source as unavailable
func MarkSourceUnavailable(ctx context.Context, req *CreateRequest, forgedApp *ForgedApplication, incomingErr error) error {
	availabilityStatus := "unavailable"
	availabilityStatusError := fmt.Sprintf("Resource Creation error: failed to create resources in Amazon. Error: %s", incomingErr)

	appID, err := strconv.ParseInt(req.ApplicationID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid application ID: %w", err)
	}

	sourceID, err := strconv.ParseInt(req.SourceID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}

	tenantID, err := getTenantIDFromExternal(req.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Build extra payload with partial state if available
	var extra map[string]interface{}
	if forgedApp != nil {
		extra = forgedApp.ApplicationExtraPayload()
	} else {
		extra = make(map[string]interface{})
	}

	extraJSON, err := json.Marshal(extra)
	if err != nil {
		return fmt.Errorf("failed to marshal extra data: %w", err)
	}

	// Update application
	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &tenantID})
	err = appDao.Update(&m.Application{
		ID:                      appID,
		AvailabilityStatus:      availabilityStatus,
		AvailabilityStatusError: availabilityStatusError,
		Extra:                   datatypes.JSON(extraJSON),
	})
	if err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}

	l.Log.WithField("application_id", appID).Info(`Application marked as "unavailable"`)

	// Update source
	sourceDao := dao.GetSourceDao(&dao.RequestParams{TenantID: &tenantID})
	err = sourceDao.Update(&m.Source{
		ID:                 sourceID,
		AvailabilityStatus: availabilityStatus,
	})
	if err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	l.Log.WithField("source_id", sourceID).Info(`Source marked as "unavailable"`)

	return nil
}

// getAWSWaitTime returns the configured wait time for AWS IAM propagation
func getAWSWaitTime() int {
	conf := config.Get()
	if conf.SuperkeyAWSWaitTime > 0 {
		return conf.SuperkeyAWSWaitTime
	}
	return DefaultAWSWaitTime
}

// getTenantIDFromExternal gets the internal tenant ID from the external tenant ID
func getTenantIDFromExternal(externalTenant string) (int64, error) {
	var tenant m.Tenant
	result := dao.DB.Where("external_tenant = ? OR org_id = ?", externalTenant, externalTenant).First(&tenant)
	if result.Error != nil {
		return 0, result.Error
	}
	return tenant.Id, nil
}
