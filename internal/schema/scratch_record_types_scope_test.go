package schema

import "testing"

func TestScratchDefaultRecordTypeAddsOnlyCapturedObjects(t *testing.T) {
	objects := map[string]*Object{}
	for _, name := range []string{"Account", "Contact", "Opportunity", "Lead", "Example__c"} {
		objects[name] = &Object{Name: name}
	}
	defaults := map[string]string{"Account": "A", "Contact": "C", "Opportunity": "O", "Lead": "L", "Example__c": "E"}
	addScratchDefaultRecordTypes(objects, defaults)
	for _, name := range []string{"Account", "Contact", "Opportunity"} {
		if len(objects[name].Fields) != 1 || objects[name].Fields[0].Name != "RecordTypeId" {
			t.Errorf("%s: expected captured RecordTypeId field", name)
		}
	}
	for _, name := range []string{"Lead", "Example__c"} {
		if len(objects[name].Fields) != 0 {
			t.Errorf("%s: added a field outside captured scope", name)
		}
	}
}
