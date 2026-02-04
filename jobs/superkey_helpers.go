package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/RedHatInsights/sources-api-go/dao"
	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
	m "github.com/RedHatInsights/sources-api-go/model"
)

// buildCreateRequest builds a provider.CreateRequest from an Application model
func buildCreateRequest(app *m.Application) (*provider.CreateRequest, error) {
	// Get application metadata (superkey steps)
	metadata, err := getApplicationSuperkeyMetaData(app.ApplicationTypeID, app.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load superkey metadata: %w", err)
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("no superkey metadata found for application type %d", app.ApplicationTypeID)
	}

	// Convert metadata to steps
	steps := make([]provider.Step, len(metadata))
	for i, md := range metadata {
		// Parse payload and substitutions from JSON
		var payload string
		var substitutions map[string]string

		if md.Payload != nil {
			json.Unmarshal(md.Payload, &payload)
		}

		if md.Substitutions != nil {
			json.Unmarshal(md.Substitutions, &substitutions)
		}

		steps[i] = provider.Step{
			Step:          md.Step,
			Name:          md.Name,
			Payload:       payload,
			Substitutions: substitutions,
		}
	}

	// Get superkey authentication ID
	superkeyAuthID, err := getSuperKeyAuthentication(app.SourceID, app.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get superkey authentication: %w", err)
	}

	// Parse extra field for additional parameters
	extra, err := getExtraValues(app.Extra)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extra field: %w", err)
	}

	// Set default provider if not specified
	providerName := "amazon"
	if p, ok := extra["provider"]; ok {
		providerName = p
	}

	request := &provider.CreateRequest{
		TenantID:        app.TenantID,
		SourceID:        app.SourceID,
		ApplicationID:   app.ID,
		ApplicationType: app.ApplicationType.Name,
		SuperKey:        superkeyAuthID,
		Provider:        providerName,
		Extra:           extra,
		SuperKeySteps:   steps,
	}

	return request, nil
}

// buildDestroyRequest builds a provider.DestroyRequest from an Application model
func buildDestroyRequest(app *m.Application) (*provider.DestroyRequest, error) {
	// Parse superkey_data to get GUID, steps, and provider
	if app.SuperkeyData == nil {
		return nil, nil // No superkey data, nothing to destroy
	}

	var superkeyData struct {
		GUID     string                       `json:"guid"`
		Steps    map[string]map[string]string `json:"steps"`
		Provider string                       `json:"provider"`
	}

	err := json.Unmarshal(app.SuperkeyData, &superkeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse superkey_data: %w", err)
	}

	// Get application metadata (superkey steps)
	metadata, err := getApplicationSuperkeyMetaData(app.ApplicationTypeID, app.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load superkey metadata: %w", err)
	}

	// Convert metadata to steps
	steps := make([]provider.Step, len(metadata))
	for i, md := range metadata {
		var payload string
		var substitutions map[string]string

		if md.Payload != nil {
			json.Unmarshal(md.Payload, &payload)
		}

		if md.Substitutions != nil {
			json.Unmarshal(md.Substitutions, &substitutions)
		}

		steps[i] = provider.Step{
			Step:          md.Step,
			Name:          md.Name,
			Payload:       payload,
			Substitutions: substitutions,
		}
	}

	// Get superkey authentication ID
	superkeyAuthID, err := getSuperKeyAuthentication(app.SourceID, app.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get superkey authentication: %w", err)
	}

	request := &provider.DestroyRequest{
		TenantID:       app.TenantID,
		SuperKey:       superkeyAuthID,
		GUID:           superkeyData.GUID,
		Provider:       superkeyData.Provider,
		StepsCompleted: superkeyData.Steps,
		SuperKeySteps:  steps,
	}

	return request, nil
}

// getApplicationSuperkeyMetaData loads metadata for superkey steps
func getApplicationSuperkeyMetaData(applicationTypeID int64, tenantID int64) ([]m.MetaData, error) {
	metaDataDao := dao.GetMetaDataDao()

	// Fetch superkey steps for this application type
	superkeyMetadata, err := metaDataDao.GetSuperKeySteps(applicationTypeID)
	if err != nil {
		return nil, err
	}

	return superkeyMetadata, nil
}

// getSuperKeyAuthentication finds the superkey authentication for a source
func getSuperKeyAuthentication(sourceID int64, tenantID int64) (int64, error) {
	authDao := dao.GetAuthenticationDao(&dao.RequestParams{TenantID: &tenantID})

	// List all authentications for this source
	authentications, _, err := authDao.ListForSource(sourceID, 100, 0, nil)
	if err != nil {
		return 0, err
	}

	// Find the cloud-meter-arn authentication
	for _, auth := range authentications {
		if auth.AuthType == "cloud-meter-arn" {
			return auth.DbID, nil
		}
	}

	return 0, fmt.Errorf("no cloud-meter-arn authentication found for source %d", sourceID)
}

// getExtraValues parses the Extra JSON field and returns a map of string values
func getExtraValues(extraJSON []byte) (map[string]string, error) {
	if extraJSON == nil {
		return make(map[string]string), nil
	}

	var extra map[string]interface{}
	err := json.Unmarshal(extraJSON, &extra)
	if err != nil {
		return nil, err
	}

	// Convert to map[string]string
	result := make(map[string]string)
	for k, v := range extra {
		if str, ok := v.(string); ok {
			result[k] = str
		} else if v != nil {
			// Convert non-string values to JSON string
			bytes, _ := json.Marshal(v)
			result[k] = string(bytes)
		}
	}

	return result, nil
}
