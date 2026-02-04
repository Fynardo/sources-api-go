package provider

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/RedHatInsights/sources-api-go/internal/superkey"
)

// generateGUID generates a short GUID for resource naming
func generateGUID() (string, error) {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// getShortName generates a short name from the application type
func getShortName(name string) string {
	return fmt.Sprintf("redhat-%s", path.Base(name))
}

// substituteInPayload performs substitutions in the payload based on the request data
func substituteInPayload(payload string, f *superkey.ForgedApplication, substitutions map[string]string) string {
	for name, sub := range substitutions {
		switch sub {
		case "get_account":
			accountNumber := f.Request.Extra["account"]
			payload = strings.ReplaceAll(payload, name, accountNumber)
		case "s3":
			s3name := f.StepsCompleted["s3"]["output"]
			payload = strings.ReplaceAll(payload, name, s3name)
		case "generate_external_id":
			externalID, ok := f.Request.Extra["external_id"]
			if ok {
				payload = strings.ReplaceAll(payload, name, externalID)
			}
		}
	}

	return payload
}
