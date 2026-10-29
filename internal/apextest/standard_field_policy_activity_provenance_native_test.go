package apextest

import (
	"fmt"
	"testing"
)

// Native membership: standard_describe_profiles/P1/org.tsv:708,2120,3199,
// 4626,10664,12224,13317,14892 and P1/ORG-RUN.txt. A synthetic project declares
// the captured field on Activity; Task and Event inherit it without requiring
// a local feature marker. Replay constructor/setter twins through each host.
func TestStandardFieldPolicyConfiguredActivityNativeProjectRouteTwins(t *testing.T) {
	metadata := map[string]string{
		"Activity/fields/CurrencyIsoCode.field-meta.xml": `<CustomField><fullName>CurrencyIsoCode</fullName><type>Picklist</type></CustomField>`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Task", "Event"} {
			for _, route := range []string{"anonymous", "named", "test-runner"} {
				for _, operation := range []string{"constructor", "static-setter"} {
					t.Run(api+"/"+object+"/"+route+"/"+operation, func(t *testing.T) {
						body := fmt.Sprintf("%s row=new %s(CurrencyIsoCode='USD'); System.assertEquals('USD',row.CurrencyIsoCode);", object, object)
						if operation == "static-setter" {
							body = fmt.Sprintf("%s row=new %s(); row.CurrencyIsoCode='USD'; System.assertEquals('USD',row.CurrencyIsoCode);", object, object)
						}
						runStandardFieldPolicyProvenanceProjectRoute(t, api, route, body, metadata)
					})
				}
			}
		}
	}
}
