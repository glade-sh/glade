package storage

import "testing"

func TestRuntimeTemplateSharesFrozenDefinitionsAndIsolatesRecords(t *testing.T) {
	org := benchmarkOrgState(2, 2)
	template := NewRuntimeTemplate(org)

	clone := template.CloneRuntimeOrg()
	account := clone.Objects["PerfObject0__c"]
	var recordID ID
	for id := range account.Records {
		recordID = id
		break
	}
	account.Records[recordID].Fields["Name"] = StringValue("Changed")
	clone.Objects["PerfObject0__c"] = account

	if got := org.Objects["PerfObject0__c"].Records[recordID].Fields["Name"].String; got == "Changed" {
		t.Fatalf("template clone shared mutable records with base org")
	}
	if !sameFieldMap(org.Objects["PerfObject0__c"].Definition.Fields, clone.Objects["PerfObject0__c"].Definition.Fields) {
		t.Fatalf("template clone did not share frozen definition fields")
	}
}

func TestRuntimeTemplateCloneRuntimeOrgUsesFreshObjectNameCache(t *testing.T) {
	org := benchmarkOrgState(1, 0)
	template := NewRuntimeTemplate(org)

	first := template.CloneRuntimeOrg()
	second := template.CloneRuntimeOrg()

	if first.objectNameCache == nil || second.objectNameCache == nil {
		t.Fatalf("runtime template clone missing object name cache")
	}
	if first.objectNameCache == org.objectNameCache || second.objectNameCache == org.objectNameCache || first.objectNameCache == second.objectNameCache {
		t.Fatalf("runtime template clones shared object name cache")
	}
}

func TestEnsureMutableObjectDefinitionClonesOnlyOneObject(t *testing.T) {
	org := benchmarkOrgState(2, 1)
	clone := NewRuntimeTemplate(org).CloneRuntimeOrg()

	_, cloned := EnsureMutableObjectDefinition(&clone, "PerfObject0__c")
	if !cloned {
		t.Fatalf("definition was not cloned")
	}
	object := clone.Objects["PerfObject0__c"]
	object.Definition.Fields["RuntimeOnly__c"] = Field{APIName: "RuntimeOnly__c", Type: FieldString}
	clone.Objects["PerfObject0__c"] = object

	if _, ok := org.Objects["PerfObject0__c"].Definition.Fields["RuntimeOnly__c"]; ok {
		t.Fatalf("base definition changed after EnsureMutableObjectDefinition")
	}
	if !sameFieldMap(org.Objects["PerfObject1__c"].Definition.Fields, clone.Objects["PerfObject1__c"].Definition.Fields) {
		t.Fatalf("unmutated object definition was cloned")
	}
}
