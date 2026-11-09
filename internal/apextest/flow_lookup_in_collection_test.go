package apextest

import "testing"

// Get Records filters accept In and NotIn against a primitive collection
// variable: the field matches when it equals any element of the collection.
func TestFlowRecordLookupInCollectionFilter(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			methods := map[string]string{}
			for _, operator := range []string{"In", "NotIn"} {
				name := "LookupCollection_" + operator
				nodes := `<recordLookups><name>Find</name><object>Account</object><filterLogic>and</filterLogic>` +
					`<filters><field>Name</field><operator>StartsWith</operator><value><stringValue>Collection seed</stringValue></value></filters>` +
					`<filters><field>Id</field><operator>` + operator + `</operator><value><elementReference>ids</elementReference></value></filters>` +
					`<getFirstRecordOnly>true</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically>` +
					`<connector><targetReference>Keep</targetReference></connector></recordLookups>` +
					`<assignments><name>Keep</name><assignmentItems><assignToReference>outcome</assignToReference><operator>Assign</operator><value><elementReference>Find.Name</elementReference></value></assignmentItems></assignments>`
				variables := flowPreservationVariable("ids", "String", true, false, true) +
					flowPreservationVariable("outcome", "String", false, true, false)
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = flowPreservationXML(api, "Find", nodes, variables)
				// In selects the listed record B; NotIn excludes it and leaves A.
				want := "Collection seed B"
				if operator == "NotIn" {
					want = "Collection seed A"
				}
				methods["lookup"+operator] = `Account a = new Account(Name='Collection seed A'); Account b = new Account(Name='Collection seed B'); insert new List<Account>{a, b};` +
					`Flow.Interview flow = Flow.Interview.createInterview('` + name + `', new Map<String, Object>{'ids' => new List<String>{(String) b.Id}});` +
					`flow.start(); System.assertEquals('` + want + `', flow.getVariableValue('outcome'));`
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
