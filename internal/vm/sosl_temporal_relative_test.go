package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSOSLTemporalRelativeLiteralRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, query := range []string{
			"[FIND 'Temporal' IN ALL FIELDS RETURNING Contact(Id WHERE CreatedDate = LAST_N_DAYS:1)]",
			`Search.query('FIND \'Temporal\' IN ALL FIELDS RETURNING Contact(Id WHERE CreatedDate = LAST_N_DAYS:1)')`,
		} {
			t.Run(api+"/"+query, func(t *testing.T) {
				source := "Contact row=new Contact(LastName='Temporal probe'); insert row; Test.setFixedSearchResults(new List<Id>{row.Id}); List<List<SObject>> matches=" + query + "; System.assertEquals(1,matches[0].size());"
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Contact")
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
