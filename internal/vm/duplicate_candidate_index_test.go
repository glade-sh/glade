package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Bulk duplicate matching must return the same errors when the stored set is
// large enough to use the operation's candidate index.
func TestDuplicateIndexedBulkRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Lead")
				object := org.Objects["Lead"]
				match := storage.MatchingRule{Object: "Lead", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}
				object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "Name", Object: "Lead", Active: true, ActionOnInsert: "Block", MatchRules: []storage.MatchingRule{match}}}
				for i := 0; i < 400; i++ {
					id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
					object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("%d", i)), "Company": storage.StringValue("Example")}})
				}
				org.Objects["Lead"] = object
				program, err := CompileAnonymousWithOptions(`
				List<Lead> rows = new List<Lead>{
					new Lead(LastName='44', Company='Example'),
					new Lead(LastName='45', Company='Example'),
					new Lead(LastName='99999', Company='Example')
				};
				List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(rows);
				System.assertEquals(1, found[0].getDuplicateResults()[0].getMatchResults()[0].getMatchRecords().size());
				System.assertEquals(1, found[1].getDuplicateResults()[0].getMatchResults()[0].getMatchRecords().size());
				System.assertEquals(0, found[2].getDuplicateResults()[0].getMatchResults()[0].getMatchRecords().size());
				Database.SaveResult[] results = Database.insert(rows, false);
				System.assertEquals(false, results[0].isSuccess());
				System.assertEquals(false, results[1].isSuccess());
				System.assertEquals('DUPLICATES_DETECTED', String.valueOf(results[1].getErrors()[0].getStatusCode()));
				System.assertEquals(true, results[2].isSuccess());
			`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
					err = machine.RegisterMethod(Method{Name: "DuplicateIndexTest.observe", ClassName: "DuplicateIndexTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program})
					if err != nil {
						t.Fatal(err)
					}
					invoke, compileErr := CompileAnonymousWithOptions("DuplicateIndexTest.observe();", CompileOptions{APIVersion: api})
					if compileErr != nil {
						t.Fatal(compileErr)
					}
					_, err = machine.ExecuteTestInClass(invoke, "DuplicateIndexTest")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
