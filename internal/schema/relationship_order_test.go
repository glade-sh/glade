package schema

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

func TestRelationshipOrderXMLAndJSONPresence(t *testing.T) {
	for _, text := range []string{"", "<relationshipOrder>0</relationshipOrder>", "<relationshipOrder>1</relationshipOrder>"} {
		var raw customFieldXML
		if err := xml.Unmarshal([]byte("<CustomField><fullName>Parent__c</fullName><type>MasterDetail</type>"+text+"</CustomField>"), &raw); err != nil {
			t.Fatal(err)
		}
		field := fieldFromXML(raw, "Parent__c")
		data, err := json.Marshal(field)
		if err != nil {
			t.Fatal(err)
		}
		var restored Field
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if text == "" {
			if field.RelationshipOrder != nil || restored.RelationshipOrder != nil {
				t.Fatal("absent order became specified")
			}
			continue
		}
		want := 0
		if text == "<relationshipOrder>1</relationshipOrder>" {
			want = 1
		}
		if field.RelationshipOrder == nil || restored.RelationshipOrder == nil || *field.RelationshipOrder != want || *restored.RelationshipOrder != want {
			t.Fatalf("order lost: %s", data)
		}
	}
}
