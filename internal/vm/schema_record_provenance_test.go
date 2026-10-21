package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func schemaShadowingVM(t *testing.T, populated bool) *VM {
	t.Helper()
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "sample"
	storage.EnsureStandardObject(&org, "EmailTemplate")
	if populated {
		state := org.Objects["EmailTemplate"]
		state.Records = map[storage.ID]storage.Record{
			"00X000000000001": {ID: "00X000000000001", Object: "EmailTemplate", Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue("ProbeTemplate"),
			}},
		}
		org.Objects["EmailTemplate"] = state
	}
	machine.SetOrg(&org)
	for _, name := range []string{"EmailTemplate", "SchemaNameProbe"} {
		if err := machine.RegisterClass(Class{Name: name, Namespace: "sample"}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func TestRecordArgumentResolutionPreservesSchemaProvenance(t *testing.T) {
	machine := schemaShadowingVM(t, true)
	rows, err := machine.executeSOQLQuery("SELECT Id, DeveloperName FROM EmailTemplate", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	record := rows.List[0]
	record.Static = "Object"
	resolved := machine.valueWithTypesResolvedInClass("sample.SchemaNameProbe", record)
	if resolved.Type != "EmailTemplate" || resolved.Runtime != record.Runtime || resolved.Static != "Object" {
		t.Fatalf("canonical query record identity changed: %#v", resolved)
	}
	if _, err := machine.coerceAssignable("Schema.EmailTemplate", resolved); err != nil {
		t.Fatal(err)
	}
	instance := Object("sample.EmailTemplate")
	instance.classInstance = true
	if _, err := machine.coerceAssignable("Schema.EmailTemplate", instance); err == nil {
		t.Fatal("local class instance became a schema record")
	}
}

func TestQueryListAssignmentPreservesSchemaProvenance(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("populated=%t", populated), func(t *testing.T) {
			machine := schemaShadowingVM(t, populated)
			rows, err := machine.executeSOQLQuery("SELECT Id FROM EmailTemplate", &Result{})
			if err != nil {
				t.Fatal(err)
			}
			rows.Runtime = rows.Type
			rows.Static = "Object"
			resolved := machine.valueWithTypesResolvedInClass("sample.SchemaNameProbe", rows)
			if resolved.Type != rows.Type || resolved.Runtime != rows.Runtime {
				t.Fatalf("canonical query list identity changed: %#v", resolved)
			}
			if _, err := machine.coerceAssignable("List<Schema.EmailTemplate>", resolved); err != nil {
				t.Fatal(err)
			}
			instances := List()
			instances.Type = "List<sample.EmailTemplate>"
			if _, err := machine.coerceAssignable("List<Schema.EmailTemplate>", instances); err == nil {
				t.Fatal("empty local class list became a schema record list")
			}
		})
	}
}
