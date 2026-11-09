package soql

import "strings"

// nullsClauseIgnoredLookups lists the standard lookup fields whose native
// ORDER BY ignores NULLS FIRST and NULLS LAST: nulls sort first ascending and
// last descending whatever the clause says. No describe, FieldDefinition or
// EntityParticle facet separates them from the lookups that honour the clause
// (Contact.ReportsToId, Opportunity.Pricebook2Id, Order.Pricebook2Id, the Field
// Service and consent objects, custom lookups), so the set is the observed one.
// Native rows: apextest testdata/conformance/nulls_order_standard_lookups
// (API 62.0 and 67.0, identical) and plain_global_describe nullOrder rows.
// Unprobed standard lookups keep the clause.
var nullsClauseIgnoredLookups = map[string]map[string]bool{
	"account":                 {"dandbcompanyid": true, "operatinghoursid": true, "parentid": true},
	"asset":                   {"parentid": true, "product2id": true},
	"campaign":                {"parentid": true},
	"case":                    {"accountid": true, "assetid": true, "contactid": true, "parentid": true},
	"commsubscriptionconsent": {"engagementchanneltypeid": true},
	"contact":                 {"accountid": true, "individualid": true},
	"contract":                {"companysignedid": true, "customersignedid": true, "pricebook2id": true},
	"emailmessage":            {"parentid": true, "relatedtoid": true, "replytoemailmessageid": true},
	"event":                   {"whatid": true, "whoid": true},
	"lead":                    {"dandbcompanyid": true, "individualid": true},
	"opportunity":             {"accountid": true, "campaignid": true, "contactid": true},
	"order":                   {"contractid": true, "shiptocontactid": true},
	"task":                    {"whatid": true, "whoid": true},
	"userrole":                {"parentroleid": true},
}

// StandardLookupIgnoresNullsClause reports whether ORDER BY on the direct
// field object.field ignores its NULLS clause natively.
func StandardLookupIgnoresNullsClause(object, field string) bool {
	return nullsClauseIgnoredLookups[strings.ToLower(object)][strings.ToLower(field)]
}
