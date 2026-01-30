package service

import (
	"encoding/json"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/dao"
	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	l "github.com/RedHatInsights/sources-api-go/logger"
	m "github.com/RedHatInsights/sources-api-go/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// loadApplication loads the application with required associations for superkey requests
func loadApplication(application *m.Application) (*m.Application, error) {
	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &application.TenantID})

	// Re-pulling from db to ensure we have the full version with preloaded relations
	app, err := appDao.GetByIdWithPreload(&application.ID, "Source", "Source.Tenant", "Tenant")
	if err != nil {
		return nil, err
	}

	return app, nil
}

// getApplicationSuperkeyMetaData returns the superkey steps from the metadata table
func getApplicationSuperkeyMetaData(application *m.Application) ([]superkey.Step, error) {
	// Fetch the metadata from the db (no tenancy required)
	mDB := dao.GetMetaDataDao()

	metadata, err := mDB.GetSuperKeySteps(application.ApplicationTypeID)
	if err != nil {
		return nil, err
	}

	steps := make([]superkey.Step, len(metadata))

	// Parse the data from db into the superkey "step" struct
	for i, step := range metadata {
		substitutions := make(map[string]string)

		err := json.Unmarshal(step.Substitutions, &substitutions)
		if err != nil {
			return nil, err
		}

		steps[i] = superkey.Step{
			Step:          step.Step,
			Name:          step.Name,
			Payload:       string(step.Payload),
			Substitutions: substitutions,
		}
	}

	return steps, nil
}

// getExtraValues returns provider-specific extra values for the superkey request
func getExtraValues(application *m.Application, provider string) (map[string]string, error) {
	extra := make(map[string]string)

	switch provider {
	case "amazon":
		// Fetch the account number for replacing in the IAM payloads
		mDB := dao.GetMetaDataDao()

		acct, err := mDB.GetSuperKeyAccountNumber(application.ApplicationTypeID)
		if err != nil {
			return nil, err
		}

		extra["account"] = acct

		// Fetch the result_type for the application_type
		atDB := dao.GetApplicationTypeDao(nil)

		authType, err := atDB.GetSuperKeyResultType(application.ApplicationTypeID, provider)
		if err != nil {
			return nil, err
		}

		extra["result_type"] = authType
		externalID := uuid.New().String()
		extra["external_id"] = externalID
	default:
		return nil, fmt.Errorf("invalid provider for superkey %v", provider)
	}

	return extra, nil
}

// getSuperKeyAuthentication returns the authentication used to communicate with the provider
func getSuperKeyAuthentication(application *m.Application) (*m.Authentication, error) {
	authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &application.TenantID})

	// Fetch auths for this source
	auths, _, err := authDao.ListForSource(application.SourceID, 100, 0, nil)
	if err != nil {
		return nil, err
	}

	// Find the auth attached to the application's source with the right authtype for superkey
	for i, auth := range auths {
		// TODO: parameterize this if we need superkey on something OTHER than amazon.
		if auth.ResourceID == application.SourceID && auth.AuthType == "access_key_secret_key" {
			return &auths[i], nil
		}
	}

	return nil, fmt.Errorf("superkey authentication not found")
}

type superKeyData struct {
	GUID           string
	Provider       string
	StepsCompleted map[string]map[string]string
}

func parseSuperKeyData(data datatypes.JSON) (*superKeyData, error) {
	if len(data) == 0 {
		return nil, nil
	}

	superkeyData := make(map[string]interface{})

	err := json.Unmarshal(data, &superkeyData)
	if err != nil {
		return nil, err
	}

	guid, ok := superkeyData["guid"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for guid %v", superkeyData["guid"])
	}

	provider, ok := superkeyData["provider"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for provider %v", superkeyData["provider"])
	}

	var stepsCompleted map[string]map[string]string

	rawSteps := superkeyData["steps"]
	l.Log.Debugf("rawSteps: %v", rawSteps)

	if rawSteps != nil {
		b, _ := json.Marshal(&rawSteps)

		err := json.Unmarshal(b, &stepsCompleted)
		if err != nil {
			l.Log.Warnf("Failed to unmarshal completed steps into map: %v", err)
		}

		l.Log.Debugf("Found stepsCompleted: %v", stepsCompleted)
	}

	return &superKeyData{
		GUID:           guid,
		Provider:       provider,
		StepsCompleted: stepsCompleted,
	}, nil
}
