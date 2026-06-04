package vm

import "strings"

func managedFeatureValueKey(kind, apiName string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + ":" + strings.ToLower(strings.TrimSpace(apiName))
}
