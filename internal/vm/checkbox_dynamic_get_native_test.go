package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned API67 named and anonymous observations distinguish dynamic checkbox
// reads (null) from dotted reads (false) on fresh, unset Contact records.
func TestSObjectDynamicCheckboxGetNative67(t *testing.T) {
	for name, setup := range map[string]string{
		"absent": `Contact row = new Contact();`,
		"withId": `Contact row = new Contact(Id='003000000000072', LastName='Owned');`,
		"withParent": `Account parent = new Account(Id='001000000000071', Name='Owned');
Contact row = new Contact(Id='003000000000071', AccountId=parent.Id, LastName='Owned');
row.putSObject('Account', parent);`,
	} {
		t.Run(name, func(t *testing.T) {
			runDynamicCheckboxNative67(t, setup+`
System.assertEquals(null, row.get('DoNotCall'));
System.assertEquals(null, row.get(Contact.DoNotCall));
Object raw = row.get('DoNotCall');
Boolean key = (Boolean)raw;
System.assertEquals(null, key);
System.assertEquals(false, row.DoNotCall);
`)
		})
	}
}

func TestSObjectDynamicCheckboxKeysNative67(t *testing.T) {
	runDynamicCheckboxNative67(t, `
Map<Boolean,Integer> counts = new Map<Boolean,Integer>();
Set<Boolean> keys = new Set<Boolean>();
for (Contact row : new List<Contact>{new Contact(DoNotCall=true), new Contact(DoNotCall=false), new Contact()}) {
    Object raw = row.get('DoNotCall');
    Boolean key = (Boolean)raw;
    keys.add(key);
    counts.put(key, counts.containsKey(key) ? counts.get(key) + 1 : 1);
}
System.assertEquals(3, counts.size());
System.assertEquals(3, keys.size());
System.assertEquals(1, counts.get(null));
System.assertEquals(1, counts.get(false));
System.assertEquals(1, counts.get(true));
System.assert(keys.contains(null));
System.assert(keys.contains(false));
System.assert(keys.contains(true));
`)
}

func TestSObjectDynamicCheckboxNative67Controls(t *testing.T) {
	runDynamicCheckboxNative67(t, `
for (Contact row : new List<Contact>{new Contact(DoNotCall=null), new Contact()}) {
    row.put('DoNotCall', null);
    System.assertEquals(null, row.get('DoNotCall'));
    System.assertEquals(null, row.get(Contact.DoNotCall));
    System.assertEquals(false, row.DoNotCall);
}
for (Boolean value : new List<Boolean>{false, true}) {
    Contact row = new Contact(DoNotCall=value);
    System.assertEquals(value, row.get('DoNotCall'));
    System.assertEquals(value, row.get(Contact.DoNotCall));
    System.assertEquals(value, row.DoNotCall);
}
OwnedCheckboxValue holder = new OwnedCheckboxValue();
Object value = holder.value;
System.assertEquals(null, holder.value);
System.assertEquals(null, (Boolean)value);
`)
}

func runDynamicCheckboxNative67(t *testing.T, source string) {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "OwnedCheckboxValue", APIVersion: "67.0", Fields: map[string]Field{
		"value": {Name: "value", Type: "Boolean", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"LastName":  {APIName: "LastName", Type: storage.FieldString},
			"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			"DoNotCall": {APIName: "DoNotCall", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// These are local trust-boundary controls, not native claims about the
// unobserved contexts. They preserve their existing read implementations.
func TestFreshDefaultedCheckboxGetScopeControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("observed/"+api, func(t *testing.T) {
			machine, row := freshDefaultedCheckboxGuardMachine(api)
			if !machine.freshDefaultedCheckboxGetIsNull(row, "DoNotCall", Bool(false)) {
				t.Fatal("observed fresh defaulted standard checkbox was excluded")
			}
		})
	}
	for _, cell := range []struct {
		name   string
		modify func(*VM, *Value, *string, *Value)
	}{
		{"source61", func(machine *VM, _ *Value, _ *string, _ *Value) { machine.currentMethod.APIVersion = "61.0" }},
		{"source68", func(machine *VM, _ *Value, _ *string, _ *Value) { machine.currentMethod.APIVersion = "68.0" }},
		{"unknownSource", func(machine *VM, _ *Value, _ *string, _ *Value) { machine.currentMethod.APIVersion = "" }},
		{"orgNamespace", func(machine *VM, _ *Value, _ *string, _ *Value) { machine.Org.Namespace = "owned" }},
		{"callerNamespace", func(machine *VM, _ *Value, _ *string, _ *Value) {
			machine.currentMethod.SourceContextBound = true
			machine.currentMethod.Namespace = "owned"
		}},
		{"unmarkedLoadedDefault", func(_ *VM, row *Value, _ *string, _ *Value) { delete(row.Fields, sobjectDefaultedFieldsField) }},
		{"explicitFalse", func(_ *VM, row *Value, _ *string, _ *Value) { markExplicitSObjectField(row, "DoNotCall") }},
		{"trueValue", func(_ *VM, _ *Value, _ *string, value *Value) { *value = Bool(true) }},
		{"nullValue", func(_ *VM, _ *Value, _ *string, value *Value) { *value = typedNull("Boolean") }},
		{"queried", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectQueriedFieldsField] = Map() }},
		{"jsonOrigin", func(_ *VM, row *Value, _ *string, _ *Value) {
			marker := Map()
			marker.Runtime = jsonDeserializedSObjectFieldOrder
			row.Fields[sobjectExplicitFieldsField] = marker
		}},
		{"cloneWithId", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectCloneMarkerField] = Bool(true) }},
		{"cloneWithoutId", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectCloneMarkerField] = Bool(false) }},
		{"dml", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectDMLAccessibleField] = Bool(true) }},
		{"trigger", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectTriggerField] = Bool(true) }},
		{"parentProjection", func(_ *VM, row *Value, _ *string, _ *Value) { row.Fields[sobjectParentProjectionField] = Bool(true) }},
		{"storedId", func(machine *VM, row *Value, _ *string, _ *Value) {
			id := storage.ID("003000000000071")
			row.Fields["Id"] = String(string(id))
			contact := machine.Org.Objects["Contact"]
			contact.Records[id] = storage.Record{ID: id, Object: "Contact"}
			machine.Org.Objects["Contact"] = contact
		}},
		{"customField", func(machine *VM, row *Value, field *string, _ *Value) {
			*field = "Owned__c"
			contact := machine.Org.Objects["Contact"]
			contact.Definition.Fields[*field] = storage.Field{APIName: *field, Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"}
			machine.Org.Objects["Contact"] = contact
			row.Fields[*field] = Bool(false)
			markDefaultedSObjectField(row, *field)
		}},
		{"formula", func(machine *VM, _ *Value, _ *string, _ *Value) {
			contact := machine.Org.Objects["Contact"]
			field := contact.Definition.Fields["DoNotCall"]
			field.Formula = "false"
			contact.Definition.Fields["DoNotCall"] = field
			machine.Org.Objects["Contact"] = contact
		}},
		{"customMetadata", func(machine *VM, _ *Value, _ *string, _ *Value) {
			contact := machine.Org.Objects["Contact"]
			contact.Definition.Metadata = map[string]string{"kind": "customMetadata"}
			machine.Org.Objects["Contact"] = contact
		}},
		{"customSetting", func(machine *VM, _ *Value, _ *string, _ *Value) {
			contact := machine.Org.Objects["Contact"]
			contact.Definition.Metadata = map[string]string{"kind": "customSetting"}
			machine.Org.Objects["Contact"] = contact
		}},
	} {
		t.Run(cell.name, func(t *testing.T) {
			machine, row := freshDefaultedCheckboxGuardMachine("67.0")
			field, value := "DoNotCall", Bool(false)
			cell.modify(machine, &row, &field, &value)
			if machine.freshDefaultedCheckboxGetIsNull(row, field, value) {
				t.Fatal("unobserved or assigned value crossed the checkbox read boundary")
			}
		})
	}
}

func freshDefaultedCheckboxGuardMachine(api string) (*VM, Value) {
	machine := New(nil)
	machine.currentMethod.APIVersion = api
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
			"DoNotCall": {APIName: "DoNotCall", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	row := Object("Contact")
	row.Fields["DoNotCall"] = Bool(false)
	markDefaultedSObjectField(&row, "DoNotCall")
	return machine, row
}
