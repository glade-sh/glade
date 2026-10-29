package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are release-behavior controls, not Salesforce assertions. The captured
// family covers populated Name stripping and Database.countQuery; it must not
// change required custom fields or other query entry routes without an oracle.
func TestDefaultPermissionsCapturePreservesUncapturedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			t.Run("selectedAbsentName", func(t *testing.T) {
				machine := fieldPermissionFallbackMachine(false)
				state := machine.Org.Objects["FallbackProbe__c"]
				state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString}
				machine.Org.Objects["FallbackProbe__c"] = state
				row := Object("FallbackProbe__c")
				row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(row.Type, map[string]bool{"name": true})
				removed := Map()
				modified, err := machine.stripInaccessibleRecord("CREATABLE", &row, false, removed, "")
				if err != nil || modified || len(removed.Map) != 0 || !machine.queriedSObjectFieldsIncludes(row, "Name") {
					t.Fatalf("absent Name release control: modified=%v removed=%v error=%v", modified, removed, err)
				}
			})
			t.Run("requiredCustomField", func(t *testing.T) {
				machine := fieldPermissionFallbackMachine(false)
				program, err := CompileAnonymousWithOptions(`
SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{new FallbackProbe__c(RequiredChoice__c='First')},false);
System.assertEquals('First',d.getRecords()[0].get('RequiredChoice__c'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			for _, route := range []struct{ name, source string }{
				{"staticCount", `Integer n=[SELECT COUNT() FROM FallbackProbe__c];`},
				{"dynamicQueryCount", `Database.query('SELECT COUNT() FROM FallbackProbe__c');`},
				{"paginationCount", `Database.getPaginationCursor('SELECT COUNT() FROM FallbackProbe__c');`},
			} {
				t.Run(route.name, func(t *testing.T) {
					machine := fieldPermissionFallbackMachine(false)
					actor := machine.executionUser
					machine.EnableTestContext()
					machine.testContext.CurrentUser = actor
					machine.testContext.RunAsDepth = 1
					source := `try {` + route.source + `return 'ok';} catch(Exception e) {return e.getTypeName();}`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					got, err := machine.callMethod(Method{Name: "PreservedPermissionRoutes.observe", ClassName: "PreservedPermissionRoutes", IsStatic: true, APIVersion: api, SourceContextBound: true, Program: program}, nil, &Result{})
					want := "ok"
					if api == "67.0" || route.name == "dynamicQueryCount" {
						want = "System.QueryException"
					}
					if err != nil || got.String() != want {
						t.Fatalf("release control=%s; got=%s error=%v", want, got.String(), err)
					}
				})
			}
		})
	}
}
