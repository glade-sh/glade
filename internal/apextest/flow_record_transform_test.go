package apextest

import "testing"

// A Transform element with an SObject, non-collection output builds one record
// from Map actions. Inside a loop it maps each item, and an Add assignment can
// collect the mapped records.
func TestFlowRecordTransformMapsLoopItem(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			nodes := `<recordLookups><name>Find</name><object>Account</object><filterLogic>and</filterLogic>` +
				`<filters><field>Name</field><operator>StartsWith</operator><value><stringValue>Transform seed</stringValue></value></filters>` +
				`<getFirstRecordOnly>false</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically>` +
				`<connector><targetReference>Each</targetReference></connector></recordLookups>` +
				`<loops><name>Each</name><collectionReference>Find</collectionReference><iterationOrder>Asc</iterationOrder>` +
				`<nextValueConnector><targetReference>ToContact</targetReference></nextValueConnector></loops>` +
				`<transforms><name>ToContact</name><connector><targetReference>Collect</targetReference></connector>` +
				`<dataType>SObject</dataType><isCollection>false</isCollection><objectType>Contact</objectType><scale>0</scale>` +
				`<transformValues>` +
				`<transformValueActions><outputFieldApiName>LastName</outputFieldApiName><transformType>Map</transformType><value><elementReference>Each.Name</elementReference></value></transformValueActions>` +
				`<transformValueActions><outputFieldApiName>AccountId</outputFieldApiName><transformType>Map</transformType><value><elementReference>Each.Id</elementReference></value></transformValueActions>` +
				`<transformValueActions><outputFieldApiName>Title</outputFieldApiName><transformType>Map</transformType><value><stringValue>Mapped</stringValue></value></transformValueActions>` +
				`</transformValues></transforms>` +
				`<assignments><name>Collect</name><assignmentItems><assignToReference>mapped</assignToReference><operator>Add</operator><value><elementReference>ToContact</elementReference></value></assignmentItems>` +
				`<connector><targetReference>Each</targetReference></connector></assignments>`
			variables := `<variables><name>mapped</name><dataType>SObject</dataType><objectType>Contact</objectType><isInput>false</isInput><isOutput>true</isOutput><isCollection>true</isCollection></variables>`
			files := map[string]string{"force-app/main/default/flows/TransformLoop.flow-meta.xml": flowPreservationXML(api, "Find", nodes, variables)}
			methods := map[string]string{"transformLoop": `Account a = new Account(Name='Transform seed A'); insert a;` +
				`Flow.Interview flow = Flow.Interview.createInterview('TransformLoop', new Map<String, Object>()); flow.start();` +
				`List<Contact> mapped = (List<Contact>) flow.getVariableValue('mapped');` +
				`System.assertEquals(1, mapped.size()); System.assertEquals('Transform seed A', mapped[0].LastName);` +
				`System.assertEquals(a.Id, mapped[0].AccountId); System.assertEquals('Mapped', mapped[0].Title);`}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
