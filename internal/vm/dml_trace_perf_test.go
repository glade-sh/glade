package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDatabaseDMLTraceDisabledAvoidsTraceRecordPreparation(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetTraceEnabled(false)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)

	result := Result{}
	record := Object("Account")
	record.Fields["Name"] = String("trace-off")
	if _, err := machine.executeDatabaseDML("insert", []Value{record}, &result); err != nil {
		t.Fatal(err)
	}

	got := recorder.Snapshot().DML
	if got.RecordConversions != 1 || got.TraceRecordPreparations != 0 {
		t.Fatalf("DML record conversions = %#v, want one operational conversion and no trace preparation", got)
	}
	if len(result.Trace) != 0 {
		t.Fatalf("trace disabled result has %d events, want none", len(result.Trace))
	}
}

func TestDMLStatementTraceDisabledAvoidsTraceRecordPreparation(t *testing.T) {
	program, err := CompileAnonymous(`insert new Account(Name = 'trace-off');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetTraceEnabled(false)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)

	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}

	got := recorder.Snapshot().DML
	if got.RecordConversions != 1 || got.TraceRecordPreparations != 0 {
		t.Fatalf("DML record conversions = %#v, want one operational conversion and no trace preparation", got)
	}
	if len(result.Trace) != 0 {
		t.Fatalf("trace disabled result has %d events, want none", len(result.Trace))
	}
}

func TestDatabaseDMLTraceEnabledPreservesRecordPayload(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)

	result := Result{traceEnabled: true}
	record := Object("Account")
	record.Fields["Name"] = String("trace-on")
	if _, err := machine.executeDatabaseDML("insert", []Value{record}, &result); err != nil {
		t.Fatal(err)
	}

	got := recorder.Snapshot().DML
	if got.RecordConversions != 2 || got.TraceRecordPreparations != 1 {
		t.Fatalf("DML record conversions = %#v, want operational and trace conversions", got)
	}
	var found bool
	for _, event := range result.Trace {
		if event.Category != "apex.dml" {
			continue
		}
		if event.Args["operation"] != "insert" || event.Args["rows"] != 1 {
			continue
		}
		objects, ok := event.Args["objects"].([]string)
		if !ok || len(objects) != 1 || objects[0] != "Account" {
			continue
		}
		found = true
	}
	if !found {
		t.Fatalf("DML trace did not retain insert/Account/one-row payload: %#v", result.Trace)
	}
}

func TestDatabaseDMLTraceDisabledPreservesConversionErrorBeforeMissingOrg(t *testing.T) {
	machine := New(nil)
	machine.SetTraceEnabled(false)

	_, err := machine.executeDatabaseDML("insert", []Value{Int(1)}, &Result{})
	if err == nil {
		t.Fatal("Database.insert accepted a non-sObject without org state")
	}
	if got := err.Error(); !strings.Contains(got, "DML requires sObject value") {
		t.Fatalf("Database.insert error = %q, want sObject conversion error before missing-org error", got)
	}
}

func TestDMLTraceTogglePreservesAliasAndOrgBehavior(t *testing.T) {
	program, err := CompileAnonymous(`
Account original = new Account(Name = 'shared');
Account alias = original;
insert original;
System.assertNotEquals(null, original.Id);
System.assertEquals(original.Id, alias.Id);
`)
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		originalID storage.ID
		aliasID    storage.ID
		storedName string
	}
	run := func(t *testing.T, traceEnabled bool) outcome {
		t.Helper()
		machine := New(nil)
		org := testDataOrg()
		machine.SetOrg(&org)
		machine.SetTraceEnabled(traceEnabled)
		result, err := machine.Execute(program)
		if err != nil {
			t.Fatal(err)
		}
		if traceEnabled && len(result.Trace) == 0 {
			t.Fatal("trace-enabled DML produced no trace events")
		}
		if !traceEnabled && len(result.Trace) != 0 {
			t.Fatalf("trace-disabled DML produced %d trace events", len(result.Trace))
		}
		originalID := sObjectIDFromFields(machine.Globals["original"].Fields)
		aliasID := sObjectIDFromFields(machine.Globals["alias"].Fields)
		record, ok := org.Objects["Account"].Records[storage.ID(originalID)]
		if !ok {
			t.Fatalf("inserted Account %q not found in org", originalID)
		}
		name, _ := record.GetField("Name")
		return outcome{originalID: originalID, aliasID: aliasID, storedName: name.String}
	}

	traceOn := run(t, true)
	traceOff := run(t, false)
	if traceOn != traceOff {
		t.Fatalf("trace toggle changed DML outcome: on=%#v off=%#v", traceOn, traceOff)
	}
}
