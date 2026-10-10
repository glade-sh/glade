package schema

import (
	"encoding/xml"
	"testing"
)

func TestFieldFromXMLPreservesReparentableMasterDetail(t *testing.T) {
	for _, test := range []struct {
		name string
		xml  string
		want bool
	}{
		{name: "absent defaults false", xml: "", want: false},
		{name: "false", xml: "<reparentableMasterDetail>false</reparentableMasterDetail>", want: false},
		{name: "true", xml: "<reparentableMasterDetail>true</reparentableMasterDetail>", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var raw customFieldXML
			if err := xml.Unmarshal([]byte("<CustomField><fullName>Parent__c</fullName><type>MasterDetail</type>"+test.xml+"</CustomField>"), &raw); err != nil {
				t.Fatal(err)
			}
			if got := fieldFromXML(raw, "Parent__c").ReparentableMasterDetail; got != test.want {
				t.Fatalf("ReparentableMasterDetail = %v, want %v", got, test.want)
			}
		})
	}
}
