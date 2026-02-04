package superkey

import (
	"strconv"

	"github.com/RedHatInsights/sources-api-go/model"
)

// ReconstructForgedApplication rebuilds a ForgedApplication from a DestroyRequest
func ReconstructForgedApplication(request *DestroyRequest) *ForgedApplication {
	return &ForgedApplication{
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

// MarkCompleted marks a step as completed with the given data
func (f *ForgedApplication) MarkCompleted(name string, data map[string]string) {
	f.StepsCompleted[name] = data
}

// CreatePayload creates and populates the Product field on the ForgedApplication
func (f *ForgedApplication) CreatePayload(username, password, appType *string) {
	authtype := f.Request.Extra["result_type"]
	resourceId, _ := strconv.ParseInt(f.Request.ApplicationID, 10, 64)

	f.Product = &App{
		SourceID: f.Request.SourceID,
		Extra:    f.ApplicationExtraPayload(),
		AuthPayload: model.AuthenticationCreateRequest{
			AuthType:      authtype,
			Username:      username,
			ResourceIDRaw: resourceId,
			ResourceType:  "Application",
		},
	}
}

// ApplicationExtraPayload builds the extra payload for the application
func (f *ForgedApplication) ApplicationExtraPayload() map[string]interface{} {
	extra := map[string]interface{}{
		"_superkey": map[string]interface{}{
			"steps":    f.StepsCompleted,
			"guid":     f.GUID,
			"provider": f.Request.Provider,
		},
	}

	// Return the s3 bucket if it was set during resource creation
	if f.StepsCompleted["s3"] != nil {
		extra["bucket"] = f.StepsCompleted["s3"]["output"]
	}

	return extra
}

// GetAuthenticationCreateRequest builds the authentication create request from the forged app
func (f *ForgedApplication) GetAuthenticationCreateRequest() *model.AuthenticationCreateRequest {
	if f.Product == nil {
		return nil
	}

	extra := map[string]interface{}{}
	externalID, ok := f.Request.Extra["external_id"]
	if ok {
		extra["external_id"] = externalID
	}

	return &model.AuthenticationCreateRequest{
		AuthType:      f.Product.AuthPayload.AuthType,
		Username:      f.Product.AuthPayload.Username,
		ResourceType:  f.Product.AuthPayload.ResourceType,
		ResourceIDRaw: f.Request.ApplicationID,
		Extra:         extra,
	}
}
