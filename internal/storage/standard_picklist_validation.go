package storage

import "strings"

// Catalog-derived restrictions are enforced only for fields with owned native
// insert and update observations. The describe facet remains available for all
// fields; it is not by itself proof of a complete DML value set.
var nativeRestrictedPicklistFields = map[string]map[string]bool{
	"account":        {"billinggeocodeaccuracy": true, "shippinggeocodeaccuracy": true},
	"contact":        {"mailinggeocodeaccuracy": true},
	"lead":           {"geocodeaccuracy": true},
	"task":           {"calltype": true, "recurrencetype": true, "recurrenceinstance": true, "recurrencemonthofyear": true},
	"opportunity":    {"forecastcategoryname": true},
	"event":          {"showas": true, "recurrencetype": true},
	"contract":       {"ownerexpirationnotice": true, "status": true, "billinggeocodeaccuracy": true},
	"order":          {"status": true, "billinggeocodeaccuracy": true},
	"quote":          {"billinggeocodeaccuracy": true},
	"contentversion": {"sharingoption": true, "sharingprivacy": true},
}

// StandardPicklistCatalogValidationExcluded limits new catalog enforcement to
// org.tsv rows 2-5 of the original and broad-sample captures at API 62/67.
func StandardPicklistCatalogValidationExcluded(objectName, fieldName string) bool {
	return !nativeRestrictedPicklistFields[strings.ToLower(objectName)][strings.ToLower(fieldName)]
}
