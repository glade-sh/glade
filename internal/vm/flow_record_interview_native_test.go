package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/storage"
)

// F06 captures createInterview(...).start() for an after-save record Flow.
// Renaming the owned metadata checks that discovery uses the declaration,
// including qualified names, rather than a fixture-specific exception.
func TestRecordFlowInterviewNativeColdAndWarmRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/platform_transaction_paths"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			raw, err := os.ReadFile(filepath.Join(fixture, "flow_paths", "api"+strings.TrimSuffix(api, ".0"), "native", route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			var want struct{ Exception string }
			for _, line := range strings.Split(string(raw), "\n") {
				cells := strings.SplitN(line, "\t", 5)
				if len(cells) == 5 && cells[0] == "F06" {
					if err := json.Unmarshal([]byte(cells[4]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want.Exception == "" {
				t.Fatal("missing native F06")
			}
			for _, name := range []string{"CaptureAsync", "BirchAsync", "sprig.BirchAsync"} {
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					idx := loadFlowRecordDeliveryIndex(t, fixture, api, "CaptureAsync")
					idx.Flows[0].Rules[0].Name = name
					org := testDataOrg()
					org.APIVersion = api
					org.Objects["CaptureRecord__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "CaptureRecord__c"}}
					automation.ApplyToOrg(&org, idx)
					machine := New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
					}
					source := fmt.Sprintf(`Boolean caught=false; try { Flow.Interview.createInterview('%s',new Map<String,Object>()).start(); } catch(Exception ex) { caught=true; System.assertEquals('%s',ex.getTypeName()); } System.assert(caught);`, name, want.Exception)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for _, state := range []string{"cold", "warm"} {
						t.Run(state, func(t *testing.T) {
							if _, err := machine.Execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
				})
			}
		}
	}
}

// These three-argument observations include the declaration fallback and the
// missing-namespace boundary, rather than assuming the two-argument route.
func TestRecordFlowInterviewNativeNamespaceColdAndWarmRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/platform_transaction_paths/flow_path_routes"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 5)
				if len(cells) != 5 {
					t.Fatal("invalid native row")
				}
				var row map[string]any
				if json.Unmarshal([]byte(cells[4]), &row) != nil {
					t.Fatal("invalid native JSON")
				}
				rows[cells[0]] = row
			}
			for _, tc := range []struct{ id, name, namespace string }{
				{"I02", "CaptureAsync", "''"}, {"I03", "CaptureAsync", "null"}, {"I04", "CaptureAsync", "'sprig'"},
				{"I05", "CaptureScheduled", "''"}, {"I06", "CaptureScheduled", "null"}, {"I07", "CaptureScheduled", "'sprig'"},
				{"I08", "BirchAsync", "''"}, {"I09", "BirchAsync", "null"}, {"I10", "BirchAsync", "'sprig'"},
			} {
				t.Run(api+"/"+route+"/"+tc.id, func(t *testing.T) {
					want := rows[tc.id]
					if want == nil || want["exception"] == nil {
						t.Fatal("missing native interview row")
					}
					// Load the exact captured declarations; no fabricated namespace metadata.
					idx := loadFlowRecordDeliveryIndex(t, fixture, api, tc.name)
					org := testDataOrg()
					org.APIVersion = api
					org.Objects["CaptureRecord__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "CaptureRecord__c"}}
					automation.ApplyToOrg(&org, idx)
					machine := New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
					}
					created := want["created"] == true
					source := fmt.Sprintf(`Boolean created=false; Boolean caught=false; try { Flow.Interview interview=Flow.Interview.createInterview(%s,'%s',new Map<String,Object>()); created=interview!=null; interview.start(); } catch(Exception ex) {caught=true;System.assertEquals('%s',ex.getTypeName());} System.assert(caught);System.assertEquals(%v,created);`, tc.namespace, tc.name, want["exception"], created)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for _, state := range []string{"cold", "warm"} {
						t.Run(state, func(t *testing.T) {
							if _, err := machine.Execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
				})
			}
		}
	}
}
