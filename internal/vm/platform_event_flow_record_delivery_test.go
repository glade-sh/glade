package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPlatformEventFlowRecordDeliveryRoutesNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_record_route_twins/delivery"
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string]map[string]any{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 3)
			if len(cells) != 3 {
				t.Fatal("invalid native delivery row")
			}
			var row map[string]any
			if err := json.Unmarshal([]byte(cells[2]), &row); err != nil {
				t.Fatal(err)
			}
			rows[cells[0]+"/"+cells[1]] = row
		}
		for _, route := range []string{"anonymous/event_committed", "istest/event_explicit", "istest/event_stop"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.Objects["BoundaryDirect__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "BoundaryDirect__e", KeyPrefix: "e00", Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
					Fields: map[string]storage.Field{"Key__c": {APIName: "Key__c", Type: storage.FieldString, Length: 80}},
				}, Records: map[storage.ID]storage.Record{}}
				idx := loadFlowRecordDeliveryIndex(t, fixture, api, "BoundaryEventDirect")
				if len(idx.Diagnostics) != 0 {
					t.Fatalf("flow import: %v", idx.Diagnostics)
				}
				automation.ApplyToOrg(&org, idx)
				machine := New(nil)
				machine.SetOrg(&org)
				run := func(source string) Result {
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
				before, after := "", ""
				if strings.HasPrefix(route, "istest/") {
					machine.EnableTestContext()
					before = "Test.startTest();"
					after = "Test.stopTest();"
				}
				if strings.HasSuffix(route, "explicit") {
					after = "Test.getEventBus().deliver();" + after
				}
				run("insert new Account(Name='Delivery unit',Phone='111');")
				result := run(before + "Database.SaveResult published=EventBus.publish(new BoundaryDirect__e(Key__c='Delivery unit')); System.assertEquals(true,published.isSuccess());" + after)
				if err := machine.DrainAsync(&result); err != nil {
					t.Fatal(err)
				}
				field := "after"
				if strings.HasPrefix(route, "anonymous/") {
					field = "phone"
				}
				phone, ok := rows[route][field].(string)
				if !ok {
					t.Fatal("missing native effect")
				}
				run(fmt.Sprintf("System.assertEquals('%s',[SELECT Phone FROM Account WHERE Name='Delivery unit' LIMIT 1].Phone);", phone))
			})
		}
	}
}

func TestScheduledFlowInterviewRoutesNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_record_route_twins/delivery"
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string]map[string]any{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 3)
			if len(cells) != 3 {
				t.Fatal("invalid native delivery row")
			}
			var row map[string]any
			if err := json.Unmarshal([]byte(cells[2]), &row); err != nil {
				t.Fatal(err)
			}
			rows[cells[0]+"/"+cells[1]] = row
		}
		for _, context := range []string{"anonymous", "istest"} {
			for _, route := range []string{"direct", "subflow", "direct_static", "subflow_static", "direct_namespace", "subflow_namespace"} {
				t.Run(api+"/"+context+"/"+route, func(t *testing.T) {
					idx := loadFlowRecordDeliveryIndex(t, fixture, api, "BoundaryScheduledDirect", "BoundaryScheduledNested", "BoundaryLeaf")
					org := testDataOrg()
					automation.ApplyToOrg(&org, idx)
					machine := New(nil)
					machine.SetOrg(&org)
					for _, class := range []Class{{Name: "Flow.Interview"}, {Name: "Flow.Interview.BoundaryScheduledDirect", SuperClass: "Flow.Interview"}, {Name: "Flow.Interview.BoundaryScheduledNested", SuperClass: "Flow.Interview"}} {
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
					}
					before, after := "", ""
					if context == "istest" {
						machine.EnableTestContext()
						before = "Test.startTest();"
						after = "Test.stopTest();"
					}
					name := "BoundaryScheduledDirect"
					if strings.HasPrefix(route, "subflow") {
						name = "BoundaryScheduledNested"
					}
					call := "Flow.Interview.createInterview('" + name + "',inputs)"
					if strings.HasSuffix(route, "_namespace") {
						call = "Flow.Interview.createInterview('','" + name + "',inputs)"
					}
					if strings.HasSuffix(route, "_static") {
						call = "new Flow.Interview." + name + "(inputs)"
					}
					row := rows[context+"/scheduled_"+route]
					phone, ok := row["after"].(string)
					if !ok {
						t.Fatal("missing native scheduled row")
					}
					output, ok := row["output"].(bool)
					if !ok {
						t.Fatal("missing native scheduled output")
					}
					source := before + "Account seed=new Account(Name='Boundary seed',Phone='111'); insert seed; Map<String,Object> inputs=new Map<String,Object>{'$Record'=>seed}; Flow.Interview interview=" + call + "; interview.start();" + after
					source += fmt.Sprintf("System.assertEquals('%s',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone); System.assertEquals(%t,interview.getVariableValue('returned') != null);", phone, output)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func loadFlowRecordDeliveryIndex(t *testing.T, fixture, api string, names ...string) automation.Index {
	t.Helper()
	p := project.Project{Root: t.TempDir(), SourceAPIVersion: api}
	for _, name := range names {
		source, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/flows", name+".flow-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(p.Root, name+".flow-meta.xml")
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "62.0", api)), 0600); err != nil {
			t.Fatal(err)
		}
		p.FlowFiles = append(p.FlowFiles, path)
	}
	idx, err := automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}
