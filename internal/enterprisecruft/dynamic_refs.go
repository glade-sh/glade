package enterprisecruft

import "strings"

type DynamicReferences struct {
	DynamicApex           bool
	CustomMetadataRouting bool
}

func DetectDynamicReferences(source string) DynamicReferences {
	low := strings.ToLower(source)
	return DynamicReferences{
		DynamicApex:           strings.Contains(low, "type.forname") || strings.Contains(low, "class.forname"),
		CustomMetadataRouting: strings.Contains(low, "__mdt") || strings.Contains(low, "getinstance(") || strings.Contains(low, "getall("),
	}
}

func (refs DynamicReferences) Any() bool {
	return refs.DynamicApex || refs.CustomMetadataRouting
}
