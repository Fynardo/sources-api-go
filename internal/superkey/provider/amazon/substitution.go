package amazon

import (
	"strings"

	"github.com/RedHatInsights/sources-api-go/internal/superkey/provider"
)

// substituteInPayload replaces template variables in the payload with actual values
// This handles special substitutions like:
// - get_account: Replace with AWS account number from request
// - s3: Replace with the S3 bucket name that was created
// - generate_external_id: Replace with external ID from request
func substituteInPayload(payload string, result *provider.Result, substitutions map[string]string) string {
	for name, sub := range substitutions {
		switch sub {
		case "get_account":
			// Get account number from the request extra data
			if accountNumber, ok := result.Request.Extra["account"]; ok {
				payload = strings.ReplaceAll(payload, name, accountNumber)
			}

		case "s3":
			// Get S3 bucket name from completed steps
			if result.StepsCompleted["s3"] != nil {
				if s3name, ok := result.StepsCompleted["s3"]["output"]; ok {
					payload = strings.ReplaceAll(payload, name, s3name)
				}
			}

		case "generate_external_id":
			// Get external ID from the request extra data
			if externalID, ok := result.Request.Extra["external_id"]; ok {
				payload = strings.ReplaceAll(payload, name, externalID)
			}
		}
	}

	return payload
}
