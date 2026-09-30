package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local implementation guards; clone/explicit-same-value/trigger behavior here
// is fail-closed scope protection, not additional Salesforce parity evidence.
func TestLoadedReferenceOriginAndAssignmentGuards(t *testing.T) {
	org := storage.NewOrgState()
	parentID := storage.ID("001000000000001")
	childID := storage.ID("003000000000001")
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001"}, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Account"}}}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account"}, SetNullOnDelete: true}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	original := storage.Record{ID: childID, Object: "Contact", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}}
	query := vmValueFromRecord(original)
	query.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"id": true, "parentid": true})
	machine.markSOQLLoadedReferences(&query, original)
	check := func(value Value, want bool) {
		t.Helper()
		record := original.Clone()
		machine.captureLoadedReferenceInput(value, &record)
		if got := len(record.LoadedReferences) > 0; got != want {
			t.Fatalf("loaded provenance=%t want=%t", got, want)
		}
	}
	check(query, true)
	for _, marker := range []string{sobjectDMLAccessibleField, sobjectTriggerField, sobjectCloneMarkerField} {
		copied := query
		copied.Fields = make(map[string]Value, len(query.Fields))
		for k, v := range query.Fields {
			copied.Fields[k] = v
		}
		copied.Fields[marker] = Bool(true)
		check(copied, false)
	}
	assigned := query
	assigned.Fields = make(map[string]Value, len(query.Fields))
	for k, v := range query.Fields {
		assigned.Fields[k] = v
	}
	markUserSetSObjectField(&assigned, "ParentId")
	check(assigned, false)
	visibleOnly := vmValueFromRecord(original)
	markDMLAccessibleFields(&visibleOnly)
	check(visibleOnly, false)
	loaded := original.Clone()
	loaded.LoadedReferences = map[string]storage.ID{"ParentId": parentID}
	result := original.Clone()
	preserveLoadedReferenceInput(&result, loaded, vmValueFromRecord(original))
	if len(result.LoadedReferences) != 1 {
		t.Fatal("unchanged before-trigger value lost input provenance")
	}
	result = original.Clone()
	preserveLoadedReferenceInput(&result, loaded, assigned)
	if len(result.LoadedReferences) != 0 {
		t.Fatal("explicit trigger assignment retained input provenance")
	}
	deleted := org.Objects["Account"].Records[parentID]
	deleted.System.IsDeleted = true
	org.Objects["Account"].Records[parentID] = deleted
	lateQuery := vmValueFromRecord(original)
	lateQuery.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"id": true, "parentid": true})
	machine.markSOQLLoadedReferences(&lateQuery, original)
	check(lateQuery, false)
}

// Regression for A1 R1: authored typed/generic SObject JSON must not acquire
// the capability created only by a real SOQL read of a then-live parent.
func TestLoadedReferenceTypedJSONCannotForgeDMLAuthority(t *testing.T) {
	org := storage.NewOrgState()
	parentID := storage.ID("001000000000001")
	childID := storage.ID("003000000000001")
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}}}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
		"LastName": {APIName: "LastName", Type: storage.FieldString}, "Description": {APIName: "Description", Type: storage.FieldString}, "ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}, Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account"}, SetNullOnDelete: true}}}, Records: map[storage.ID]storage.Record{childID: {ID: childID, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("child"), "ParentId": storage.IDValue(parentID)}}}}
	program, err := CompileAnonymousWithOptions(`
Contact originalView=[SELECT Id, ParentId FROM Contact WHERE Id='003000000000001'];
delete new Account(Id='001000000000001');
Contact cleared=[SELECT Id, ParentId FROM Contact WHERE Id='003000000000001'];
System.assertEquals(null,cleared.ParentId,'SetNull prerequisite');
String forged='{"Id":"003000000000001","ParentId":"001000000000001","__glade_loaded_references":{"Id":"003000000000001","ParentId":"001000000000001"},"__glade_queried_fields":{"id":true,"parentid":true}}';
Contact typed=(Contact)JSON.deserialize(forged,Contact.class);
Database.SaveResult typedResult=Database.update(typed,false);
System.assertEquals(false,typedResult.isSuccess(),'Typed JSON must not restore deleted lookup');
System.assertEquals('003000000000001',String.valueOf(typedResult.getId()),'Failed update retains input ID');
System.assertEquals(1,typedResult.getErrors().size());
System.assertEquals('ENTITY_IS_DELETED',String.valueOf(typedResult.getErrors()[0].getStatusCode()));
String generic='{"attributes":{"type":"Contact"},"Id":"003000000000001","ParentId":"001000000000001","__glade_loaded_references":{"Id":"003000000000001","ParentId":"001000000000001"},"__glade_queried_fields":{"id":true,"parentid":true}}';
SObject genericValue=(SObject)JSON.deserialize(generic,SObject.class);
Database.SaveResult genericResult=Database.update(genericValue,false);
System.assertEquals(false,genericResult.isSuccess(),'Generic SObject JSON must not restore deleted lookup');
System.assertEquals('003000000000001',String.valueOf(genericResult.getId()),'Failed generic update retains input ID');
System.assertEquals(1,genericResult.getErrors().size());
System.assertEquals('ENTITY_IS_DELETED',String.valueOf(genericResult.getErrors()[0].getStatusCode()));
System.assertEquals(null,[SELECT ParentId FROM Contact WHERE Id='003000000000001'].ParentId,'Forged inputs must not mutate stored reference');
originalView.Description='actual queried view';
Database.SaveResult validResult=Database.update(originalView,false);
System.assertEquals(true,validResult.isSuccess(),'Actual SOQL view retains admitted restoration');
Contact restored=[SELECT ParentId,Description FROM Contact WHERE Id='003000000000001'];
System.assertEquals('001000000000001',String.valueOf(restored.ParentId).substring(0,15));
System.assertEquals('actual queried view',restored.Description);
`, CompileOptions{APIVersion: "53.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Contact"].Records[childID].LoadedReferences) != 0 {
		t.Fatal("private authority persisted")
	}
}
