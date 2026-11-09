// Code generated from standard_api68_describe.json.gz by scripts/generate-standard-api68-new-objects.py; DO NOT EDIT.

package storage

// standardAPI68NewObjectNames lists the objects some captured profile
// described with status 200 at API 68. Keys are lower-cased API names.
var standardAPI68NewObjectNames = map[string]struct{}{
	"addresschangeevent":                 {},
	"associatedlocationhistory":          {},
	"asyncthirdpartypaymentgatewayevent": {},
	"contactpointottsvc":                 {},
	"contactpointottsvcshare":            {},
	"dominoappactivityevent":             {},
	"floworchestrationinstancelink":      {},
	"floworchestrationinstancelinkshare": {},
	"floworchestrationworkitemevent":     {},
	"fulfillmentorderhistory":            {},
	"fulfillmentorderlineitemhistory":    {},
	"mcpserverconfigurationevent":        {},
	"messagingsessionclient":             {},
	"outboundengagementtemplate":         {},
	"outboundengagementtemplateshare":    {},
	"securitydefenseactionevent":         {},
	"shiftallocationrequest":             {},
	"shiftallocationrequestfeed":         {},
	"shiftallocationrequesthistory":      {},
	"webcartactivitylogchangeevent":      {},
	"webcartcollaboratorchangeevent":     {},
	"wishlistitemrelationship":           {},
}

// standardAPI68ProfileFeatures is each captured profile's feature list, in
// sidecar order. StandardAPI68DescribeProfile derives signatures from it.
var standardAPI68ProfileFeatures = map[string][]string{
	"P0":  {"__glade_edition_developer"},
	"P1":  {"__glade_edition_developer", "MultiCurrency"},
	"P10": {"__glade_edition_developer", "B2BCommerce", "Communities", "CommerceEnabled"},
	"P11": {"__glade_edition_enterprise"},
	"P2":  {"__glade_edition_developer", "PersonAccounts"},
	"P3":  {"__glade_edition_developer", "StateAndCountryPicklist"},
	"P4":  {"__glade_edition_developer", "Communities"},
	"P5":  {"__glade_edition_developer", "Quotes"},
	"P6":  {"__glade_edition_developer", "ProductsAndSchedules"},
	"P7":  {"__glade_edition_developer", "Entitlements"},
	"P8":  {"__glade_edition_developer", "OrderManagement"},
	"P9":  {"__glade_edition_developer", "ContactsToMultipleAccounts"},
	"PU":  {"__glade_edition_developer", "Communities", "Entitlements", "MultiCurrency", "OrderManagement", "PersonAccounts", "ProductsAndSchedules", "SalesUser", "ServiceCloud", "StateAndCountryPicklist", "Quotes"},
}
