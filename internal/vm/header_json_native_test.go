package vm

import (
	"fmt"
	"testing"
)

// Owned named source-67/transport-62 observation 0AfRu00000hbmbBKAQ:
// header JSON accepts these twelve properties, preserves Long milliseconds,
// and supplies numeric zero/empty-list defaults for omitted properties.
// The constructor and namespace contexts were not captured.
func TestExecHeaderJSONNative(t *testing.T) {
	const populated = `{"entityName":"Account","changeType":"UPDATE","changeOrigin":"owned","transactionKey":"owned-tx","sequenceNumber":7,"commitTimestamp":1700000000123,"commitUser":"005000000000001AAA","commitNumber":42,"recordIds":["001000000000001AAA","001000000000002AAA"],"changedFields":["Name"],"nulledFields":["Industry"],"diffFields":["Description"]}`
	for _, test := range []struct{ name, body string }{
		{"populated", fmt.Sprintf(`
EventBus.ChangeEventHeader header = (EventBus.ChangeEventHeader)JSON.deserialize('%s',EventBus.ChangeEventHeader.class);
System.assertEquals('Account',header.entityName);
System.assertEquals('UPDATE',header.changeType);
System.assertEquals('owned',header.changeOrigin);
System.assertEquals('owned-tx',header.transactionKey);
System.assertEquals(7,header.sequenceNumber);
System.assertEquals(1700000000123L,header.commitTimestamp);
System.assertEquals('005000000000001AAA',header.commitUser);
System.assertEquals(42L,header.commitNumber);
System.assertEquals(new List<String>{'001000000000001AAA','001000000000002AAA'},header.recordIds);
System.assertEquals(new List<String>{'Name'},header.changedFields);
System.assertEquals(new List<String>{'Industry'},header.nulledFields);
System.assertEquals(new List<String>{'Description'},header.diffFields);
`, populated)},
		{"empty", `
EventBus.ChangeEventHeader header = (EventBus.ChangeEventHeader)JSON.deserialize('{}',EventBus.ChangeEventHeader.class);
System.assertEquals(null,header.entityName);
System.assertEquals(null,header.changeType);
System.assertEquals(null,header.changeOrigin);
System.assertEquals(null,header.transactionKey);
System.assertEquals(0,header.sequenceNumber);
System.assertEquals(0L,header.commitTimestamp);
System.assertEquals(null,header.commitUser);
System.assertEquals(0L,header.commitNumber);
System.assertEquals(new List<String>(),header.recordIds);
System.assertEquals(new List<String>(),header.changedFields);
System.assertEquals(new List<String>(),header.nulledFields);
System.assertEquals(new List<String>(),header.diffFields);
`},
		{"null", `System.assertEquals(null,JSON.deserialize('null',EventBus.ChangeEventHeader.class));`},
		{"strictUnknown", `
Boolean caught = false;
try {JSON.deserializeStrict('{"ownedUnknownProperty":1}',EventBus.ChangeEventHeader.class);}
catch(JSONException issue) {
    caught = true;
    System.assertEquals('Unrecognized field "ownedUnknownProperty" at [line:1, column:26]',issue.getMessage());
}
System.assert(caught);
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.body, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecHeaderJSONNativeKeepsControls(t *testing.T) {
	machine := New(nil)
	fields := map[string]Field{}
	for name, typ := range map[string]string{
		"entityName": "String", "sequenceNumber": "Integer", "commitTimestamp": "Long", "recordIds": "List<String>",
	} {
		fields[name] = Field{Name: name, Type: typ}
	}
	if err := machine.RegisterClass(Class{Name: "OwnedHeaderControl", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymousWithOptions(`
OwnedHeaderControl plain = (OwnedHeaderControl)JSON.deserialize('{"entityName":"Account","sequenceNumber":7,"commitTimestamp":1700000000123,"recordIds":["001000000000001AAA"]}',OwnedHeaderControl.class);
System.assertEquals('Account',plain.entityName);
System.assertEquals(7,plain.sequenceNumber);
System.assertEquals(1700000000123L,plain.commitTimestamp);
System.assertEquals(new List<String>{'001000000000001AAA'},plain.recordIds);
Database.SaveResult result = (Database.SaveResult)JSON.deserialize('{"success":true,"id":"001000000000001AAA","errors":[]}',Database.SaveResult.class);
System.assertEquals(true,result.isSuccess());
System.assertEquals('001000000000001AAA',result.getId());
System.assertEquals(0,result.getErrors().size());
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHeaderJSONRejectsUnknownNative(t *testing.T) {
	for _, method := range []string{"deserialize", "deserializeStrict"} {
		t.Run(method, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Boolean caught = false;
try {JSON.%s('{"ownedUnknownProperty":1}',EventBus.ChangeEventHeader.class);}
catch(JSONException issue) {
    caught = true;
    System.assertEquals('Unrecognized field "ownedUnknownProperty" at [line:1, column:26]',issue.getMessage());
}
System.assert(caught);
`, method), CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// These local dispatch controls distinguish source classes and linked platform
// stubs. They do not claim native namespace or constructor parity.
func TestHeaderJSONKeepsSourceClassShadow(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "EventBus"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "EventBus.ChangeEventHeader", Fields: map[string]Field{
		"commitTimestamp": {Name: "commitTimestamp", Type: "String"},
		"recordIds":       {Name: "recordIds", Type: "String"},
	}}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.typedValueFromJSON("EventBus.ChangeEventHeader", map[string]any{
		"commitTimestamp": "source epoch", "recordIds": "source records", "entityName": "undeclared",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.Fields["commitTimestamp"].Text != "source epoch" || value.Fields["recordIds"].Text != "source records" {
		t.Fatalf("source field types changed: %#v", value.Fields)
	}
	if _, present := value.Fields["entityName"]; present {
		t.Fatal("platform field leaked into source class")
	}
	empty, err := machine.typedValueFromJSON("EventBus.ChangeEventHeader", map[string]any{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Fields["commitTimestamp"].Kind != ValueNull || empty.Fields["recordIds"].Kind != ValueNull {
		t.Fatalf("platform defaults leaked into source class: %#v", empty.Fields)
	}
}

func TestHeaderJSONDefaultsWithLinkedPlatformStub(t *testing.T) {
	for _, constructor := range []bool{false, true} {
		t.Run(fmt.Sprintf("constructor=%t", constructor), func(t *testing.T) {
			machine := New(nil)
			class := Class{Name: "EventBus.ChangeEventHeader", PlatformType: true, Fields: map[string]Field{
				"sequenceNumber":  {Name: "sequenceNumber", Type: "Integer"},
				"commitTimestamp": {Name: "commitTimestamp", Type: "Long"},
				"recordIds":       {Name: "recordIds", Type: "List<String>"},
			}}
			if constructor {
				class.Constructors = []Method{{Name: "EventBus.ChangeEventHeader.<init>", ClassName: "EventBus.ChangeEventHeader", IsConstructor: true}}
			}
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			value, err := machine.typedValueFromJSON("EventBus.ChangeEventHeader", map[string]any{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if value.Fields["sequenceNumber"].Kind != ValueInt || value.Fields["sequenceNumber"].Int != 0 || value.Fields["commitTimestamp"].Kind != ValueInt || value.Fields["commitTimestamp"].Int != 0 || value.Fields["recordIds"].Kind != ValueList || len(value.Fields["recordIds"].List) != 0 {
				t.Fatalf("linked platform defaults differ: %#v", value.Fields)
			}
		})
	}
}
