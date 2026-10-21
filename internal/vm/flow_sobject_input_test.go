package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowSObjectInputBindsReferencedRecord(t *testing.T) {
	for _, source := range []string{"$Record", "$Record__Prior", "Lookup_Record"} {
		t.Run(source, func(t *testing.T) {
			machine, method := flowSObjectInputMachine(t, "RecordCopyAction", "List<Account>", `
LastInput = records;
List<Account> copies = records.deepClone(false, false, false);
insert copies;
System.assertEquals('Selected seed', copies[0].Name);
`)
			selected := flowSObjectInputRecord("Selected seed")
			current := flowSObjectInputRecord("Current seed")
			context := dml.FlowActionContext{Records: map[string]storage.Record{"$record": current}}
			switch source {
			case "$Record":
				context.Records["$record"] = selected
			case "$Record__Prior":
				context.Records["$record__prior"] = selected
			case "Lookup_Record":
				context.LookupOutputs = map[string]storage.Record{"lookup_record": selected}
			}
			action := storage.FlowAction{
				Name: "Copy_Record", ClassName: method.ClassName, MethodName: "copyRecords",
				Inputs: []storage.WorkflowFieldUpdate{{Name: "records", SourceField: source}},
			}
			if err := machine.invokeFlowActionWithContext(action, current, context, &Result{}); err != nil {
				t.Fatal(err)
			}
			input, _, ok := machine.lookupStaticField(method.ClassName, "LastInput")
			if !ok || input.Value.Kind != ValueList || len(input.Value.List) != 1 {
				t.Fatalf("invocable input = %#v", input.Value)
			}
			item := input.Value.List[0]
			if item.Type != "Account" || item.Fields["Name"].Text != "Selected seed" {
				t.Fatalf("record input lost its concrete value: %#v", item)
			}
			if _, exists := item.Fields["records"]; exists {
				t.Fatal("parameter name became a synthetic SObject field")
			}
			assertFlowSObjectInputCopy(t, machine)
		})
	}
}

func TestFlowSObjectInputPreservesRequestWrapper(t *testing.T) {
	machine, method := flowSObjectInputMachine(t, "WrappedRecordCopyAction", "List<RecordInput>", `
System.assertEquals('Selected seed', records[0].records.Name);
List<Account> copies = new List<Account>{records[0].records}.deepClone(false, false, false);
insert copies;
`)
	if err := machine.RegisterClass(Class{
		Name: "WrappedRecordCopyAction.RecordInput",
		Fields: map[string]Field{
			"records": {Name: "records", Type: "Account", Access: "public", Modifiers: []string{"InvocableVariable"}},
		},
		FieldOrder: []string{"records"},
	}); err != nil {
		t.Fatal(err)
	}
	record := flowSObjectInputRecord("Selected seed")
	action := storage.FlowAction{
		Name: "Copy_Wrapped_Record", ClassName: method.ClassName, MethodName: "copyRecords",
		Inputs: []storage.WorkflowFieldUpdate{{Name: "records", SourceField: "$Record"}},
	}
	context := dml.FlowActionContext{Records: map[string]storage.Record{"$record": record}}
	if err := machine.invokeFlowActionWithContext(action, record, context, &Result{}); err != nil {
		t.Fatal(err)
	}
	assertFlowSObjectInputCopy(t, machine)
}

func TestFlowSObjectInputDirectMethodControl(t *testing.T) {
	machine, method := flowSObjectInputMachine(t, "DirectRecordCopyAction", "List<Account>", `
System.assertEquals('Selected seed', records[0].Name);
List<Account> copies = records.deepClone(false, false, false);
insert copies;
`)
	input := typedList("List<Account>")
	input.List = append(input.List, machine.vmValueFromRecord(flowSObjectInputRecord("Selected seed")))
	if _, err := machine.callMethod(method, []Value{input}, &Result{}); err != nil {
		t.Fatal(err)
	}
	assertFlowSObjectInputCopy(t, machine)
}

func TestFlowSObjectInputRetainsUnknownAuthoredFieldRejection(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	row := Object("Account")
	row.Fields["Name"] = String("Selected seed")
	row.Fields["Unrecognized__c"] = String("authored")
	results, err := machine.applyDML("insert", row, false, "", dml.Options{}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Success {
		t.Fatalf("unknown authored field was accepted: %#v", results)
	}
	if len(machine.Org.Objects["Account"].Records) != 0 {
		t.Fatal("invalid record was persisted")
	}
}

func flowSObjectInputMachine(t *testing.T, className, parameterType, body string) (*VM, Method) {
	t.Helper()
	program, err := CompileAnonymous(body)
	if err != nil {
		t.Fatal(err)
	}
	method := Method{
		Name: className + ".copyRecords", ClassName: className, ReturnType: "void",
		Params:   []Param{{Name: "records", Type: parameterType}},
		IsStatic: true, Access: "public", Modifiers: []string{"InvocableMethod"}, Program: program,
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: className,
		StaticFields: map[string]Field{
			"LastInput": {Name: "LastInput", Type: parameterType, Static: true},
		},
		Methods: map[string]Method{"copyRecords": method},
	}); err != nil {
		t.Fatal(err)
	}
	return machine, method
}

func flowSObjectInputRecord(name string) storage.Record {
	return storage.Record{
		ID: "001000000000031", Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue(name)},
	}
}

func assertFlowSObjectInputCopy(t *testing.T, machine *VM) {
	t.Helper()
	rows := machine.Org.Objects["Account"].Records
	if len(rows) != 1 {
		t.Fatalf("inserted record count = %d", len(rows))
	}
	for id, row := range rows {
		if id == "001000000000031" || row.Fields["Name"].String != "Selected seed" {
			t.Fatalf("copy lost its Name or retained the source Id: %#v", row)
		}
		if _, exists := row.Fields["records"]; exists {
			t.Fatal("synthetic parameter field reached storage")
		}
	}
}
