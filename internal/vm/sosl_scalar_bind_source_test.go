package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the local bind behavior outside this ticket's literal-only oracle.
// Native conformance separately covers literal numeric picklist and Id filters.
func TestSOSLNumericScalarBindSource(t *testing.T) {
	for _, predicate := range []string{"Industry = :criterion", "Id = :criterion"} {
		for _, inline := range []bool{false, true} {
			name := predicate + "/dynamic"
			expression := `Search.query('FIND \'quotednumericnomatch\' RETURNING Account(Id WHERE ` + predicate + `)')`
			if inline {
				name = predicate + "/inline"
				expression = `[FIND 'quotednumericnomatch' RETURNING Account(Id WHERE ` + predicate + `)]`
			}
			t.Run(name, func(t *testing.T) {
				program, err := CompileAnonymous(`Integer criterion=3; List<List<SObject>> groups=` + expression + `; System.assertEquals(0,groups[0].size());`)
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := storage.NewOrgState()
				definition, ok := storage.StandardObjectDefinition("Account")
				if !ok {
					t.Fatal("missing standard Account schema")
				}
				org.Objects["Account"] = storage.ObjectState{Definition: definition}
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
