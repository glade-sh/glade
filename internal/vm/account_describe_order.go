package vm

import (
	"sort"
	"strings"
)

// The Account field map lists the 59 captured standard fields in the platform
// order (default_order_rest d3 rows, APIs 62.0 and 67.0, anonymous and test
// route); any field the capture does not hold follows, sorted. Namespaced orgs
// and other objects keep the sorted order.
var capturedAccountDescribeFieldOrder = []string{
	"id", "isdeleted", "masterrecordid", "name", "type", "parentid",
	"billingstreet", "billingcity", "billingstate", "billingpostalcode", "billingcountry", "billinglatitude",
	"billinglongitude", "billinggeocodeaccuracy", "billingaddress", "shippingstreet", "shippingcity", "shippingstate",
	"shippingpostalcode", "shippingcountry", "shippinglatitude", "shippinglongitude", "shippinggeocodeaccuracy", "shippingaddress",
	"phone", "fax", "accountnumber", "website", "photourl", "sic",
	"industry", "annualrevenue", "numberofemployees", "ownership", "tickersymbol", "description",
	"rating", "site", "ownerid", "createddate", "createdbyid", "lastmodifieddate",
	"lastmodifiedbyid", "systemmodstamp", "lastactivitydate", "lastvieweddate", "lastreferenceddate", "jigsaw",
	"jigsawcompanyid", "cleanstatus", "accountsource", "dunsnumber", "tradestyle", "naicscode",
	"naicsdesc", "yearstarted", "sicdesc", "dandbcompanyid", "operatinghoursid",
}

func (vm *VM) sortCapturedAccountDescribeFieldNames(objectName string, names []string) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || !strings.EqualFold(objectName, "Account") {
		return false
	}
	ordinals := make(map[string]int, len(capturedAccountDescribeFieldOrder))
	for index, name := range capturedAccountDescribeFieldOrder {
		ordinals[name] = index
	}
	sort.SliceStable(names, func(i, j int) bool {
		left, leftKnown := ordinals[strings.ToLower(names[i])]
		right, rightKnown := ordinals[strings.ToLower(names[j])]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			return left < right
		}
		return names[i] < names[j]
	})
	return true
}
