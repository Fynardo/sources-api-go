package service

import (
	"context"
	"strconv"

	"github.com/RedHatInsights/sources-api-go/config"
	"github.com/RedHatInsights/sources-api-go/dao"
	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
	"github.com/RedHatInsights/sources-api-go/kafka"
	l "github.com/RedHatInsights/sources-api-go/logger"
	m "github.com/RedHatInsights/sources-api-go/model"
	"github.com/sirupsen/logrus"
)

// SendSuperKeyCreateRequest initiates the superkey application creation process
func SendSuperKeyCreateRequest(application *m.Application, headers []kafka.Header) error {
	conf := config.Get()

	// Check if superkey creation is disabled
	if conf.SuperkeyDisableCreation {
		l.Log.Info("Superkey creation is disabled, skipping")
		return nil
	}

	// Build the create request
	req, err := buildCreateRequest(application)
	if err != nil {
		return err
	}

	// Get the superkey authentication credentials
	superKeyAuth, err := getSuperKeyAuthentication(application)
	if err != nil {
		return err
	}

	// Get the password from the authentication
	password, err := superKeyAuth.GetPassword()
	if err != nil {
		return err
	}

	// Extract step names for client initialization
	stepNames := make([]string, len(req.SuperKeySteps))
	for i, step := range req.SuperKeySteps {
		stepNames[i] = step.Name
	}

	// Run the superkey creation asynchronously
	go func() {
		ctx := context.Background()

		l.Log.WithFields(logrus.Fields{
			"application_id":   req.ApplicationID,
			"application_type": req.ApplicationType,
			"provider":         req.Provider,
		}).Info("Starting superkey resource creation")

		// Create the provider with credentials
		prov, err := provider.NewAmazonProviderWithCredentials(ctx, *superKeyAuth.Username, *password, stepNames)
		if err != nil {
			l.Log.WithFields(logrus.Fields{
				"application_id": req.ApplicationID,
				"error":          err,
			}).Error("Failed to create provider")

			if markErr := superkey.MarkSourceUnavailable(ctx, req, nil, err); markErr != nil {
				l.Log.WithField("error", markErr).Error("Failed to mark source unavailable")
			}
			return
		}

		// Process the create request
		result := superkey.ProcessCreate(ctx, req, prov)
		if result.Error != nil {
			l.Log.WithFields(logrus.Fields{
				"application_id": req.ApplicationID,
				"error":          result.Error,
			}).Error("Superkey create failed")

			if markErr := superkey.MarkSourceUnavailable(ctx, req, result.ForgedApp, result.Error); markErr != nil {
				l.Log.WithField("error", markErr).Error("Failed to mark source unavailable")
			}
			return
		}

		// Persist the forged application data
		if err := superkey.PersistForgedApplication(ctx, result.ForgedApp); err != nil {
			l.Log.WithFields(logrus.Fields{
				"application_id": req.ApplicationID,
				"error":          err,
			}).Error("Failed to persist forged application")

			if markErr := superkey.MarkSourceUnavailable(ctx, req, result.ForgedApp, err); markErr != nil {
				l.Log.WithField("error", markErr).Error("Failed to mark source unavailable")
			}
			return
		}

		l.Log.WithFields(logrus.Fields{
			"application_id": req.ApplicationID,
			"guid":           result.ForgedApp.GUID,
		}).Info("Superkey resource creation completed successfully")
	}()

	return nil
}

// SendSuperKeyDeleteRequest initiates the superkey application destruction process
func SendSuperKeyDeleteRequest(application *m.Application, headers []kafka.Header) error {
	conf := config.Get()

	// Check if superkey deletion is disabled
	if conf.SuperkeyDisableDeletion {
		l.Log.Info("Superkey deletion is disabled, skipping")
		return nil
	}

	// Build the destroy request
	req, err := buildDestroyRequest(application)
	if err != nil {
		return err
	}

	if req == nil {
		// No superkey data to destroy
		return nil
	}

	// Get the superkey authentication credentials
	superKeyAuth, err := getSuperKeyAuthentication(application)
	if err != nil {
		l.Log.Warnf("SuperKey Authentication was nil - cleaning up incomplete superkey")
		return nil
	}

	// Get the password from the authentication
	password, err := superKeyAuth.GetPassword()
	if err != nil {
		l.Log.Warnf("Failed to get SuperKey password: %v", err)
		return nil
	}

	// Extract step names for client initialization
	stepNames := make([]string, len(req.SuperKeySteps))
	for i, step := range req.SuperKeySteps {
		stepNames[i] = step.Name
	}

	// Run the superkey destruction asynchronously
	go func() {
		ctx := context.Background()

		l.Log.WithFields(logrus.Fields{
			"guid":     req.GUID,
			"provider": req.Provider,
		}).Info("Starting superkey resource destruction")

		// Create the provider with credentials
		prov, err := provider.NewAmazonProviderWithCredentials(ctx, *superKeyAuth.Username, *password, stepNames)
		if err != nil {
			l.Log.WithFields(logrus.Fields{
				"guid":  req.GUID,
				"error": err,
			}).Error("Failed to create provider for teardown")
			return
		}

		// Process the destroy request
		result := superkey.ProcessDestroy(ctx, req, prov)
		if len(result.TearDownErrors) > 0 {
			for _, tdErr := range result.TearDownErrors {
				l.Log.WithFields(logrus.Fields{
					"guid":  req.GUID,
					"error": tdErr,
				}).Warn("Error during superkey teardown")
			}
		} else {
			l.Log.WithField("guid", req.GUID).Info("Superkey resource destruction completed successfully")
		}
	}()

	return nil
}

// buildCreateRequest constructs a CreateRequest from an application
func buildCreateRequest(application *m.Application) (*superkey.CreateRequest, error) {
	// Load up the application with associations
	application, err := loadApplication(application)
	if err != nil {
		return nil, err
	}

	// Fetch the metadata and transform it
	steps, err := getApplicationSuperkeyMetaData(application)
	if err != nil {
		return nil, err
	}

	// Fetch the provider name from the static cache
	providerName := dao.Static.GetSourceTypeName(application.Source.SourceTypeID)

	// Fetch the extra values for this superkey request
	extra, err := getExtraValues(application, providerName)
	if err != nil {
		return nil, err
	}

	// Fetch the superkey authentication
	superKey, err := getSuperKeyAuthentication(application)
	if err != nil {
		return nil, err
	}

	superKeyId := superKey.GetID()

	return &superkey.CreateRequest{
		TenantID:        application.Tenant.ExternalTenant,
		SourceID:        strconv.FormatInt(application.SourceID, 10),
		ApplicationID:   strconv.FormatInt(application.ID, 10),
		ApplicationType: dao.Static.GetApplicationTypeName(application.ApplicationTypeID),
		SuperKey:        superKeyId,
		Provider:        providerName,
		Extra:           extra,
		SuperKeySteps:   steps,
	}, nil
}

// buildDestroyRequest constructs a DestroyRequest from an application
func buildDestroyRequest(application *m.Application) (*superkey.DestroyRequest, error) {
	// Load up the application with associations
	application, err := loadApplication(application)
	if err != nil {
		return nil, err
	}

	// Fetch the metadata and transform it
	steps, err := getApplicationSuperkeyMetaData(application)
	if err != nil {
		return nil, err
	}

	// Parse out the existing data
	skData, err := parseSuperKeyData(application.SuperkeyData)
	if err != nil {
		return nil, err
	}

	if skData == nil {
		l.Log.Warnf("SuperKey Data was nil - cleaning up incomplete superkey")
		return nil, nil
	}

	// Fetch the superkey authentication
	superKey, err := getSuperKeyAuthentication(application)
	if err != nil {
		l.Log.Warnf("SuperKey Authentication was nil - cleaning up incomplete superkey")
		return nil, nil
	}

	superKeyId := superKey.GetID()

	return &superkey.DestroyRequest{
		TenantID:       application.Tenant.ExternalTenant,
		SuperKey:       superKeyId,
		GUID:           skData.GUID,
		Provider:       skData.Provider,
		StepsCompleted: skData.StepsCompleted,
		SuperKeySteps:  steps,
	}, nil
}

// BuildDestroyRequest is exported for use by jobs package
func BuildDestroyRequest(application *m.Application) (*superkey.DestroyRequest, error) {
	return buildDestroyRequest(application)
}
