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
	"apextestresult":          {"apexlogid": true},
	"authprovider":            {"executionuserid": true, "flowdefaultaccountid": true, "flowdefaultprofileid": true, "pluginid": true, "registrationhandlerid": true},
	"asset":                   {"accountid": true, "assetprovidedbyid": true, "assetservicedbyid": true, "contactid": true, "locationid": true, "parentid": true, "product2id": true},
	"assetattribute":          {"attributepicklistvalueid": true},
	"campaign":                {"parentid": true},
	"categorynode":            {"parentid": true},
	"case":                    {"accountid": true, "assetid": true, "contactid": true, "parentid": true},
	"collaborationgroup":      {"announcementid": true},
	"commsubscriptionconsent": {"engagementchanneltypeid": true},
	"contact":                 {"accountid": true, "individualid": true},
	"contentdistribution":     {"relatedrecordid": true},
	"contentversion":          {"contentbodyid": true, "externaldatasourceid": true, "firstpublishlocationid": true},
	"custombrandasset":        {"assetsourceid": true},
	"conversationentry":       {"actorid": true},
	"contract":                {"companysignedid": true, "customersignedid": true, "pricebook2id": true},
	"emailmessage":            {"activityid": true, "emailtemplateid": true, "fromid": true, "parentid": true, "relatedtoid": true, "replytoemailmessageid": true},
	"emailmessagerelation":    {"relationid": true},
	"emailtemplate":           {"brandtemplateid": true, "enhancedletterheadid": true},
	"entitlementtemplate":     {"businesshoursid": true, "slaprocessid": true},
	"event":                   {"whatid": true, "whoid": true},
	"feeditem":                {"relatedrecordid": true},
	"lead":                    {"dandbcompanyid": true, "individualid": true},
	"opportunity":             {"accountid": true, "campaignid": true, "contactid": true},
	"order":                   {"billtocontactid": true, "companyauthorizedbyid": true, "contractid": true, "customerauthorizedbyid": true, "originalorderid": true, "shiptocontactid": true},
	"permissionsetassignment": {"permissionsetgroupid": true},
	"partner":                 {"accountfromid": true, "opportunityid": true},
	"product2":                {"externaldatasourceid": true},
	"task":                    {"whatid": true, "whoid": true},
	"user":                    {"contactid": true, "delegatedapproverid": true, "individualid": true, "managerid": true, "userroleid": true},
	"userrole":                {"parentroleid": true},
}

// StandardLookupIgnoresNullsClause reports whether ORDER BY on the direct
// field object.field ignores its NULLS clause natively.
func StandardLookupIgnoresNullsClause(object, field string) bool {
	return nullsClauseIgnoredLookups[strings.ToLower(object)][strings.ToLower(field)]
}
