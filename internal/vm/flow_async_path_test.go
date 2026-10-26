package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowAsyncPathTransactionDrainAnonymousAndIsTestAPIRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_async_paths"
	for _, api := range []string{"62.0", "67.0"} {
		readRows := func(route string) map[string]map[string]any {
			t.Helper()
			raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 3)
				if len(cells) != 3 || cells[1] != route {
					t.Fatal("invalid native route")
				}
				var row map[string]any
				if json.Unmarshal([]byte(cells[2]), &row) != nil {
					t.Fatal("invalid native observation")
				}
				rows[cells[0]] = row
			}
			return rows
		}
		committed := readRows("committed_anonymous")["COMMIT"]
		named := readRows("istest")
		for _, route := range []string{"anonymous_committed", "anonymous_test_only_boundary", "istest_insert_inside_start_stop", "istest_insert_before_start_stop", "istest_rollback_before_stop"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "FlowInterview")
				storage.EnsureStandardObject(&org, "AsyncApexJob")
				org.Objects["AsyncParcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "AsyncParcel__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
					"Name":         {APIName: "Name", Type: storage.FieldString, Length: 80},
					"Source__c":    {APIName: "Source__c", Type: storage.FieldString, Length: 80},
					"Copied__c":    {APIName: "Copied__c", Type: storage.FieldString, Length: 80},
					"Immediate__c": {APIName: "Immediate__c", Type: storage.FieldBoolean},
					"Delayed__c":   {APIName: "Delayed__c", Type: storage.FieldBoolean},
				}}, Records: map[storage.ID]storage.Record{}}
				idx := loadFlowRecordDeliveryIndex(t, fixture, api, "ParcelAsync")
				if len(idx.Diagnostics) != 0 {
					t.Fatalf("Flow import: %v", idx.Diagnostics)
				}
				automation.ApplyToOrg(&org, idx)
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "anonymous_test_only_boundary" {
					program, err := CompileAnonymousWithOptions("Test.startTest();", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					var runtimeErr *RuntimeError
					rows := readRows("anonymous")
					if !errors.As(err, &runtimeErr) || runtimeErr.Type != rows["SP02"]["exception"] || runtimeErr.Message != rows["SP02"]["message"] {
						t.Fatalf("native=%v; Glade=%v", rows["SP02"], err)
					}
					return
				}
				execute := func(source string) Result {
					t.Helper()
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				insert := "insert new AsyncParcel__c(Name='async',Source__c='initial');"
				if route == "anonymous_committed" {
					body, err := os.ReadFile(filepath.Join(fixture, "enqueue.apex"))
					if err != nil {
						t.Fatal(err)
					}
					result := execute(string(body))
					execute(fmt.Sprintf("System.assertEquals(%v,[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true]);", committed["before"]))
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					execute(fmt.Sprintf("AsyncParcel__c row=[SELECT Delayed__c,Immediate__c,Copied__c FROM AsyncParcel__c LIMIT 1];System.assertEquals(%v,row.Delayed__c);System.assertEquals(%v,row.Immediate__c);System.assertEquals('%v',row.Copied__c);System.assertEquals(%v,[SELECT count() FROM AsyncApexJob]);System.assertEquals(%v,[SELECT count() FROM FlowInterview]);", committed["delayed"], committed["immediate"], committed["copied"], committed["jobs"], committed["interviews"]))
					return
				}
				machine.EnableTestContext()
				key := "SP02"
				source := "Test.startTest();" + insert + "Test.stopTest();"
				if route == "istest_insert_before_start_stop" {
					key = "SP03"
					source = insert + "Test.startTest();Test.stopTest();"
				}
				if route == "istest_rollback_before_stop" {
					key = "SP07"
					source = "Test.startTest();Savepoint point=Database.setSavepoint();" + insert + "Database.rollback(point);Test.stopTest();"
				}
				execute(source)
				execute(fmt.Sprintf("System.assertEquals(%v,[SELECT count() FROM AsyncParcel__c]);System.assertEquals(%v,[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true]);System.assertEquals(%v,[SELECT count() FROM AsyncApexJob]);System.assertEquals(%v,[SELECT count() FROM FlowInterview]);", named[key]["rows"], named[key]["delayed"], named[key]["jobs"], named[key]["interviews"]))
			})
		}
	}
}
