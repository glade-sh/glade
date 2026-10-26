package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowAsyncPathContinuationRollbackAnonymousAndIsTestAPIRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_async_path_continuations"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"committed_anonymous", "istest"} {
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
					t.Fatal("invalid native row")
				}
				rows[cells[0]] = row
			}
			for _, key := range []string{"EXECUTE", "ROLLBACK"} {
				t.Run(api+"/"+route+"/"+key, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					org.Objects["AsyncParcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "AsyncParcel__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
						"Name": {APIName: "Name", Type: storage.FieldString, Length: 80}, "Source__c": {APIName: "Source__c", Type: storage.FieldString, Length: 80}, "Copied__c": {APIName: "Copied__c", Type: storage.FieldString, Length: 80},
						"Immediate__c": {APIName: "Immediate__c", Type: storage.FieldBoolean, DefaultValue: "false"}, "Delayed__c": {APIName: "Delayed__c", Type: storage.FieldBoolean, DefaultValue: "false"},
					}}, Records: map[storage.ID]storage.Record{}}
					idx := loadFlowRecordDeliveryIndex(t, fixture, api, "ParcelAsync", "ParcelAsyncUpdate", "ParcelContinuation")
					if len(idx.Diagnostics) != 0 {
						t.Fatalf("Flow import: %v", idx.Diagnostics)
					}
					automation.ApplyToOrg(&org, idx)
					machine := New(nil)
					machine.SetOrg(&org)
					trigger, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/triggers/AsyncContinuation.trigger"))
					if err != nil {
						t.Fatal(err)
					}
					_, body, ok := strings.Cut(string(trigger), "{")
					if !ok {
						t.Fatal("missing trigger body")
					}
					body = strings.TrimSuffix(strings.TrimSpace(body), "}")
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{Name: "AsyncContinuation", Object: "AsyncParcel__c", Timing: triggerTimingAfter, Operation: "update", APIVersion: api, Program: program}); err != nil {
						t.Fatal(err)
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
					input := func(name string) string {
						t.Helper()
						body, err := os.ReadFile(filepath.Join(fixture, name))
						if err != nil {
							t.Fatal(err)
						}
						return string(body)
					}
					if route == "istest" {
						machine.EnableTestContext()
					}
					if key == "ROLLBACK" {
						execute(input("seed.apex"))
					}
					if route == "istest" {
						execute("Test.startTest();")
					}
					name := "execute.apex"
					if key == "ROLLBACK" {
						name = "rollback.apex"
					}
					result := execute(input(name))
					if route == "istest" {
						execute("Test.stopTest();")
					} else if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					result = execute(input("readback.apex"))
					for _, line := range result.Debug {
						if _, payload, ok := strings.Cut(line, "CONTINUATION|"); ok {
							var got map[string]any
							if json.Unmarshal([]byte(payload), &got) != nil {
								t.Fatal("invalid observation")
							}
							if !reflect.DeepEqual(got, rows[key]) {
								t.Fatalf("native=%v; Glade=%v", rows[key], got)
							}
							return
						}
					}
					t.Fatal("missing observation")
				})
			}
		}
	}
}
