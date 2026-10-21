package vm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCustomHistoryPreservesOrglessStandardDescribeCRUD(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult descriptor=Account.SObjectType.getDescribe();
System.assertEquals(true,descriptor.isCreateable());
System.assertEquals(true,descriptor.isUpdateable());
System.assertEquals(true,descriptor.isDeletable());
System.assertEquals(true,descriptor.isQueryable());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if machine.Org != nil {
		t.Fatal("control requires an org-less VM")
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("ordinary standard describe without an org: %v", err)
	}
}

func TestCustomHistoryTestSeedSourceBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/test=%v", api, testContext), func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod = Method{APIVersion: api}
				if testContext {
					machine.EnableTestContext()
				}
				want := testContext && api != "67.0" && api != "68.0"
				if got := machine.allowTestHistoryInsert(); got != want {
					t.Fatalf("test history seed capability=%v, want %v", got, want)
				}
			})
		}
	}
	for _, tc := range []struct {
		name   string
		method string
		stack  []string
		want   bool
	}{
		{"older caller and helper", "64.0", []string{"62.0", "64.0"}, true},
		{"newer caller and older helper", "64.0", []string{"67.0", "64.0"}, false},
		{"older caller and newer helper", "67.0", []string{"64.0", "67.0"}, false},
		{"newer intermediate helper", "64.0", []string{"64.0", "67.0", "64.0"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			machine.currentMethod = Method{APIVersion: tc.method}
			for _, api := range tc.stack {
				machine.callStack = append(machine.callStack, callFrame{APIVersion: api})
			}
			if got := machine.allowTestHistoryInsert(); got != tc.want {
				t.Fatalf("active source stack capability=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCustomHistoryUnsupportedOperationBoundary(t *testing.T) {
	// These are local held-surface controls. They make no native claim about
	// the exception semantics of uncaptured history upsert or undelete calls.
	org := storage.NewOrgState()
	org.Objects["TrackedParcel__History"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:  "TrackedParcel__History",
		Metadata: map[string]string{"kind": "generatedHistory", "parentObject": "TrackedParcel__c"},
	}}
	org.Objects["PlainParcel__History"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "PlainParcel__History"}}
	machine := New(nil)
	machine.Org = &org
	machine.EnableTestContext()
	machine.currentMethod = Method{APIVersion: "64.0"}
	for _, operation := range []string{"upsert", "undelete"} {
		err := machine.generatedHistoryDMLBoundary(operation, []storage.Record{{Object: "TrackedParcel__History"}})
		var runtimeErr *RuntimeError
		if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
			t.Fatalf("held generated history %s boundary: %v", operation, err)
		}
		if err := machine.generatedHistoryDMLBoundary(operation, []storage.Record{{Object: "PlainParcel__History"}}); err != nil {
			t.Fatalf("suffix-only ordinary %s boundary changed: %v", operation, err)
		}
	}
}
