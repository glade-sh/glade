package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native compile succeeds but execution fails: case289_followups/static-adjacent.tsv.
func TestObjectPermissionsStaticOrderRuntime(t *testing.T) {
	for _, api := range []string{"63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			storage.EnsureStandardObject(&org, "ObjectPermissions")
			machine := New(nil)
			machine.SetOrg(&org)
			prog, err := CompileAnonymousWithOptions(`List<ObjectPermissions> r=[SELECT Id FROM ObjectPermissions ORDER BY PermissionsViewAllFields];`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(prog)
			var fault *RuntimeError
			if !errors.As(err, &fault) || fault.Type != "System.UnexpectedException" || fault.ExceptionMessage() != "field 'PermissionsViewAllFields' can not be sorted in a query call" {
				t.Fatalf("native runtime rejection: %v", err)
			}
		})
	}
}

// Native grouping compiles but fails at execution: case289_followups/static-query.tsv.
func TestObjectPermissionsStaticGroupRuntime(t *testing.T) {
	cases := map[string]string{
		"group":  `List<AggregateResult> r=[SELECT PermissionsViewAllFields, COUNT(Id) FROM ObjectPermissions GROUP BY PermissionsViewAllFields];`,
		"having": `List<AggregateResult> r=[SELECT PermissionsViewAllFields, COUNT(Id) FROM ObjectPermissions GROUP BY PermissionsViewAllFields HAVING PermissionsViewAllFields = true];`,
	}
	for _, api := range []string{"63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		for name, source := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "ObjectPermissions")
				machine := New(nil)
				machine.SetOrg(&org)
				prog, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(prog)
				var fault *RuntimeError
				if !errors.As(err, &fault) || fault.Type != "System.UnexpectedException" || fault.ExceptionMessage() != "field 'PermissionsViewAllFields' can not be grouped in a query call" {
					t.Fatalf("native runtime rejection: %v", err)
				}
			})
		}
	}
}
