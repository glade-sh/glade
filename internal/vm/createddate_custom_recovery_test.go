package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// Exercise imported object categories and copied payloads. These exclusions are
// scope boundaries, not native success claims for unobserved custom categories.
func createdDateCustomImportedOrg(t *testing.T) storage.OrgState {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`,
	}
	for name, extra := range map[string]string{
		"DateBoundary__c":   "",
		"DateSetting__c":    "<customSettingsType>List</customSettingsType>",
		"DateMetadata__mdt": "",
		"DateEvent__e":      "<publishBehavior>PublishImmediately</publishBehavior>",
		"DateBig__b":        "",
		"Contact":           "",
	} {
		files["force-app/main/default/objects/"+name+"/"+name+".object-meta.xml"] = `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Date Boundary</label><pluralLabel>Date Boundaries</pluralLabel><nameField><label>Boundary Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel>` + extra + `</CustomObject>`
		files["force-app/main/default/objects/"+name+"/fields/Editable__c.field-meta.xml"] = `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Editable__c</fullName><label>Editable</label><type>Text</type><length>40</length></CustomField>`
	}
	files["force-app/main/default/objects/UnknownBoundary__c/UnknownBoundary__c.object-meta.xml"] = `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Unknown Boundary</label><pluralLabel>Unknown Boundaries</pluralLabel></CustomObject>`
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	for name, describe := range sobject.BuildDescribeRegistry(imported).Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
	}
	// An __c token alone must not acquire positive category metadata.
	if org.Objects["DateBoundary__c"].Definition.Metadata["nameFieldType"] != "Text" ||
		org.Objects["UnknownBoundary__c"].Definition.Metadata["nameFieldType"] != "" {
		t.Fatal("fixture did not retain authored NameField metadata")
	}
	return org
}

func TestCreatedDateCustomSystemPayloadBoundaries(t *testing.T) {
	org := createdDateCustomImportedOrg(t)
	for _, cell := range []struct {
		name, object, op, mode string
		modify                 func(*Value, *storage.Record)
		filter                 bool
	}{
		{name: "systemInsert", op: "insert", mode: "SYSTEM_MODE", filter: true},
		{name: "systemUpdate", op: "update", mode: "SYSTEM_MODE", filter: true},
		{name: "userInsert", op: "insert", mode: "USER_MODE"},
		{name: "userUpdate", op: "update", mode: "USER_MODE"},
		{name: "unknownInsert", op: "insert"},
		{name: "unknownUpdate", op: "update"},
		{name: "otherStandard", object: "Contact", op: "insert", mode: "SYSTEM_MODE"},
		{name: "setting", object: "DateSetting__c", op: "insert", mode: "SYSTEM_MODE"},
		{name: "metadata", object: "DateMetadata__mdt", op: "insert", mode: "SYSTEM_MODE"},
		{name: "event", object: "DateEvent__e", op: "insert", mode: "SYSTEM_MODE"},
		{name: "bigObject", object: "DateBig__b", op: "insert", mode: "SYSTEM_MODE"},
		{name: "suffixWithoutCategory", object: "UnknownBoundary__c", op: "insert", mode: "SYSTEM_MODE"},
		{name: "cloneMarker", op: "insert", mode: "SYSTEM_MODE", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectCloneMarkerField] = Bool(false) }},
		{name: "manualAuditAssignment", op: "update", mode: "SYSTEM_MODE", modify: func(row *Value, _ *storage.Record) { markUserSetSObjectField(row, "CreatedDate") }},
		{name: "copiedMismatch", op: "update", mode: "SYSTEM_MODE", modify: func(_ *Value, record *storage.Record) {
			record.Fields["CreatedDate"] = storage.DateTimeValue("2002-03-04T05:06:07.000Z")
		}},
		{name: "nullTimestamp", op: "update", mode: "SYSTEM_MODE", modify: func(row *Value, record *storage.Record) {
			row.Fields["CreatedDate"] = typedNull("Datetime")
			delete(record.Fields, "CreatedDate")
			record.ExplicitNulls["CreatedDate"] = true
		}},
		{name: "queryWithoutDMLAccess", op: "update", mode: "SYSTEM_MODE", modify: func(row *Value, _ *storage.Record) { row.Fields[sobjectDMLAccessibleField] = Bool(false) }},
	} {
		t.Run(cell.name, func(t *testing.T) {
			state := org.Clone()
			machine := New(nil)
			machine.SetOrg(&state)
			object := cell.object
			if object == "" {
				object = "DateBoundary__c"
			}
			row := Object(object)
			row.Fields["Name"] = String("Owned category boundary")
			row.Fields["CreatedDate"] = platformScalar("Datetime", "2001-02-03T04:05:06.000Z")
			row.Fields["LastModifiedDate"] = platformScalar("Datetime", "2002-03-04T05:06:07.000Z")
			markExplicitSObjectField(&row, "CreatedDate")
			markExplicitSObjectField(&row, "LastModifiedDate")
			machine.markJSONDeserializedSObjectFields(&row, row.Type)
			markUserSetSObjectField(&row, "Name")
			if cell.op == "update" {
				row.Fields["Id"] = platformScalar("Id", "a00000000000001AAA")
				markExplicitSObjectField(&row, "Id")
				row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(object, map[string]bool{"name": true})
				row.Fields[sobjectDMLAccessibleField] = Bool(true)
			}
			records, _, err := machine.recordsFromValue(row)
			if err != nil {
				t.Fatal(err)
			}
			if cell.modify != nil {
				cell.modify(&row, &records[0])
			}
			before := records[0].Clone()
			caller := cloneValuePreserveRefs(row)
			machine.filterJSONCreationTimestamp(cell.op, records, []*Value{&row}, cell.mode)
			if cell.filter {
				delete(before.Fields, "CreatedDate")
				delete(before.ExplicitNulls, "CreatedDate")
			}
			if !reflect.DeepEqual(before, records[0]) {
				t.Fatalf("copied payload outside supported timestamp change: want=%#v got=%#v", before, records[0])
			}
			if !reflect.DeepEqual(caller, row) {
				t.Fatal("filter changed caller fields or provenance")
			}
		})
	}
}

// Original operation mode must survive the same operation's chunk/retry calls,
// even though its source API defaults to user mode. Trigger bodies still resolve
// any fresh DML of their own through the ordinary entry points.
func TestCreatedDateCustomSystemModeSurvivesTriggerRecursion(t *testing.T) {
	for _, cell := range []struct {
		name          string
		count, stored int
		retry         bool
	}{
		{name: "chunks", count: 201, stored: 201},
		{name: "partialRetry", count: 2, stored: 1, retry: true},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := createdDateCustomImportedOrg(t)
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{ClassName: "OwnedModeCaller", APIVersion: "67.0"}
			body := `Integer observed = Trigger.new.size();`
			if cell.retry {
				body = `for(DateBoundary__c row : Trigger.new) { if(row.Name == 'Block') row.addError('owned retry'); }`
			}
			trigger, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "67.0", Trigger: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterTrigger(Trigger{Name: "OwnedModeBoundary", Object: "DateBoundary__c", Timing: triggerTimingBefore, Operation: "insert", APIVersion: "67.0", Program: trigger}); err != nil {
				t.Fatal(err)
			}
			first := `System.assertEquals(true,results[0].isSuccess());`
			if cell.retry {
				first = `System.assertEquals(false,results[0].isSuccess()); System.assertEquals(true,results[1].isSuccess());`
			}
			source := fmt.Sprintf(`
List<DateBoundary__c> rows = new List<DateBoundary__c>();
for(Integer i=0;i<%d;i++) {
 DateBoundary__c row=(DateBoundary__c)JSON.deserialize('{"attributes":{"type":"DateBoundary__c"},"Id":null,"Name":"Owned row","CreatedDate":"2001-02-03T04:05:06.000Z"}',DateBoundary__c.class);
 row.Name = %t && i==0 ? 'Block' : 'Owned row';
 rows.add(row);
}
List<Database.SaveResult> results=Database.insert(rows,false,AccessLevel.SYSTEM_MODE);
System.assertEquals(%d,results.size());
%s
`, cell.count, cell.retry, cell.count, first)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := len(org.Objects["DateBoundary__c"].Records); got != cell.stored {
				t.Fatalf("stored %d rows, want %d", got, cell.stored)
			}
		})
	}
}

// Construct real trigger views, then verify projection separately from all
// record fields and provenance. Same Id with another Ref is not membership.
func TestCreatedDateGeneratedTriggerProjectionBoundaries(t *testing.T) {
	for _, cell := range []struct {
		name, object, timing, operation string
		modify                          func(*VM, *Value)
		populated                       bool
	}{
		{name: "afterInsert", timing: "after", operation: "insert", populated: true},
		{name: "beforeUpdate", timing: "before", operation: "update", populated: true},
		{name: "afterUpdate", timing: "after", operation: "update", populated: true},
		{name: "beforeInsert", timing: "before", operation: "insert"},
		{name: "delete", timing: "before", operation: "delete"},
		{name: "undelete", timing: "after", operation: "undelete"},
		{name: "setting", object: "DateSetting__c", timing: "after", operation: "insert"},
		{name: "withoutCategory", object: "UnknownBoundary__c", timing: "after", operation: "insert"},
		{name: "oldReference", timing: "before", operation: "update", modify: func(vm *VM, row *Value) { *row = vm.triggerGlobals["Trigger.old"].List[0] }},
		{name: "otherReferenceSameId", timing: "after", operation: "insert", modify: func(_ *VM, row *Value) { row.Ref = newValueRef() }},
		{name: "zeroReference", timing: "after", operation: "insert", modify: func(_ *VM, row *Value) { row.Ref = 0 }},
		{name: "callerValue", timing: "after", operation: "insert", modify: func(_ *VM, row *Value) { delete(row.Fields, sobjectTriggerField) }},
		{name: "missingNewMap", timing: "after", operation: "insert", modify: func(vm *VM, _ *Value) { delete(vm.triggerGlobals, "Trigger.newMap") }},
		{name: "malformedDatetime", timing: "after", operation: "insert", modify: func(_ *VM, row *Value) { row.Fields["CreatedDate"] = platformScalar("Datetime", "invalid") }},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := createdDateCustomImportedOrg(t)
			machine := New(nil)
			machine.SetOrg(&org)
			object := cell.object
			if object == "" {
				object = "DateBoundary__c"
			}
			record := storage.Record{Object: object, ID: "a00000000000001AAA", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned trigger projection")}, System: storage.SystemFields{CreatedDate: "2002-03-04T05:06:07.000Z", LastModifiedDate: "2002-03-04T05:06:07.000Z", SystemModstamp: "2002-03-04T05:06:07.000Z"}}
			if cell.operation == "insert" && cell.timing == "before" {
				record.ID = ""
				record.System = storage.SystemFields{}
			}
			trigger := Trigger{Object: object, Timing: cell.timing, Operation: cell.operation}
			machine.triggerGlobals = triggerContext(trigger, []storage.Record{record}, []storage.Record{record})
			row := machine.triggerGlobals["Trigger.new"].List
			var receiver Value
			if len(row) > 0 {
				receiver = row[0]
			} else {
				receiver = machine.triggerGlobals["Trigger.old"].List[0]
			}
			if cell.modify != nil {
				cell.modify(machine, &receiver)
			}
			payloadBefore, beforeErr := machine.recordFromValue(&receiver)
			callerBefore := cloneValuePreserveRefs(receiver)
			oldBefore := cloneValuePreserveRefs(machine.triggerGlobals["Trigger.old"])
			populated, handled, err := machine.callSObjectMember(receiver, "getPopulatedFieldsAsMap", nil)
			if err != nil || !handled {
				t.Fatalf("getPopulatedFieldsAsMap: handled=%v error=%v", handled, err)
			}
			_, found := populated.Map[mapKey(String("CreatedDate"))]
			if found != cell.populated {
				t.Fatalf("CreatedDate populated=%v, want %v", found, cell.populated)
			}
			for _, field := range []string{"LastModifiedDate", "SystemModstamp"} {
				if _, found := populated.Map[mapKey(String(field))]; found {
					t.Fatalf("widened unobserved audit field %s", field)
				}
			}
			payloadAfter, afterErr := machine.recordFromValue(&receiver)
			if (beforeErr == nil) != (afterErr == nil) || beforeErr != nil && beforeErr.Error() != afterErr.Error() {
				t.Fatal("read projection changed DML conversion boundary")
			}
			if !reflect.DeepEqual(callerBefore, receiver) || !reflect.DeepEqual(oldBefore, machine.triggerGlobals["Trigger.old"]) || !reflect.DeepEqual(payloadBefore, payloadAfter) {
				t.Fatal("read projection changed record fields, provenance, old view or copied payload")
			}
			if cell.name == "beforeInsert" {
				if _, _, present := objectFieldValue(receiver, "CreatedDate"); present {
					t.Fatal("before insert synthesized CreatedDate")
				}
			}
		})
	}
}
