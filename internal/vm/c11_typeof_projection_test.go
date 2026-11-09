package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestC11StandardTypeofProjection(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/c11sharing_typeof/anonymous.apex")
	if err != nil {
		t.Fatal(err)
	}
	checks := `
System.assertEquals('C11 synthetic typeof',out.get('task_name'));
System.assertEquals('C11 synthetic typeof',out.get('event_name'));
System.assertEquals('System.QueryException',out.get('invalid_target'));`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			for _, name := range []string{"Account", "Task", "Event"} {
				storage.EnsureStandardObject(&org, name)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			body, err := CompileAnonymousWithOptions(string(source)+checks, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(body); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestC11CustomEventTypeofProjection(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/c11sharing_typeof_event/anonymous.apex")
	if err != nil {
		t.Fatal(err)
	}
	checks := `
System.assertEquals('C11 event custom',out.get('targetField'));
System.assertEquals('System.QueryException',out.get('disabledTarget'));`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			for _, name := range []string{"Event"} {
				storage.EnsureStandardObject(&org, name)
			}
			for _, tc := range []struct{ name, prefix, enabled string }{{"DootSecQ5B1031__c", "a40", "true"}, {"DootSecQ3B1031__c", "a41", "false"}} {
				org.Objects[tc.name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: tc.name, KeyPrefix: tc.prefix, Metadata: map[string]string{"enableActivities": tc.enabled}, Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			body, err := CompileAnonymousWithOptions(string(source)+checks, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(body); err != nil {
				t.Fatal(err)
			}
		})
	}
}
