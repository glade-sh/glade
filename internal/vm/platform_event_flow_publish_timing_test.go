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

func TestPlatformEventFlowPublishTimingAnonymousAndIsTestAPIRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/platform_transaction_paths"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous_publish", "anonymous_committed", "istest_publish", "istest_deliver", "istest_stop"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				nativeRoute, id := "anonymous", "P01"
				if strings.HasPrefix(route, "istest") {
					nativeRoute = "istest"
				}
				if route == "istest_deliver" {
					id = "P02"
				}
				if route == "istest_stop" {
					id = "P07"
				}
				raw, err := os.ReadFile(filepath.Join(fixture, "event_flows", "api"+strings.TrimSuffix(api, ".0"), "native", nativeRoute, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				var want map[string]any
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					cells := strings.SplitN(line, "\t", 5)
					if cells[0] == id {
						if err := json.Unmarshal([]byte(cells[4]), &want); err != nil {
							t.Fatal(err)
						}
					}
				}
				if want == nil {
					t.Fatal("missing native timing row")
				}
				org := testDataOrg()
				org.APIVersion = api
				org.Objects["CapturePulse__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "CapturePulse__e", KeyPrefix: "e00", Metadata: map[string]string{"publishBehavior": "PublishImmediately"}, Fields: map[string]storage.Field{
					"Key__c": {APIName: "Key__c", Type: storage.FieldString, Length: 80}, "RunKey__c": {APIName: "RunKey__c", Type: storage.FieldString, Length: 80},
				}}, Records: map[storage.ID]storage.Record{}}
				org.Objects["CaptureLog__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "CaptureLog__c", KeyPrefix: "a00", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString, Length: 80}}}, Records: map[storage.ID]storage.Record{}}
				idx := loadFlowRecordDeliveryIndex(t, fixture, api, "CaptureEvent")
				// Load the same native-deployed subscriber metadata used by the project test.
				automation.ApplyToOrg(&org, idx)
				machine := New(nil)
				machine.SetOrg(&org)
				if nativeRoute == "istest" {
					machine.EnableTestContext()
				}
				before, after := "", ""
				if route == "istest_stop" {
					before = "Test.startTest();"
					after = "Test.stopTest();"
				}
				if route == "istest_deliver" {
					after = "Test.getEventBus().deliver();"
				}
				source := fmt.Sprintf(`%s Database.SaveResult published=EventBus.publish(new CapturePulse__e(Key__c='accept',RunKey__c='timing')); System.assertEquals(true,published.isSuccess()); System.assertEquals(%v,[SELECT count() FROM CaptureLog__c]); %s System.assertEquals(%v,[SELECT count() FROM CaptureLog__c]);`, before, want["before"], after, want["after"])
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if route == "anonymous_committed" {
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					if got := len(org.Objects["CaptureLog__c"].Records); got != 1 {
						t.Fatalf("committed delivery count=%d, want 1 (native delivery P08)", got)
					}
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					if got := len(org.Objects["CaptureLog__c"].Records); got != 1 {
						t.Fatalf("repeat delivery count=%d", got)
					}
				}
			})
		}
	}
}

func TestPlatformEventFlowAnonymousQueueSnapshotAndSubscriberControls(t *testing.T) {
	for _, route := range []string{"active_flow", "inactive_flow", "no_flow"} {
		t.Run("anonymous_committed/"+route, func(t *testing.T) {
			machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
			machine.testContext = nil
			event := machine.Org.Objects["OwnedSignal__e"]
			if route != "no_flow" {
				event.Definition.FlowRules = []storage.FlowRule{{Name: "QueuedSubscriber", Active: route == "active_flow", TriggerType: "PlatformEvent", Steps: []storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "Write", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "Token__c"}}}}}}}
			}
			machine.Org.Objects["OwnedSignal__e"] = event
			records := []storage.Record{{Object: "OwnedSignal__e", Fields: map[string]storage.Value{"Token__c": storage.StringValue("Original payload")}}}
			beforeAccounts := len(machine.Org.Objects["Account"].Records)
			beforeJobs := len(machine.Org.Objects["AsyncApexJob"].Records)
			result := Result{}
			if err := machine.deliverPublishedPlatformEventSubscribers(records, &result); err != nil {
				t.Fatal(err)
			}
			active := route == "active_flow"
			if machine.HasPendingAsyncWork() != active {
				t.Fatalf("pending Flow work=%t, want %t", machine.HasPendingAsyncWork(), active)
			}
			if got := len(machine.Org.Objects["Account"].Records); got != beforeAccounts {
				t.Fatal("subscriber ran inside publisher")
			}
			records[0].Fields["Token__c"] = storage.StringValue("Changed after publication")
			if clone := machine.CloneRuntime(nil); clone.HasPendingAsyncWork() {
				t.Fatal("runtime clone inherited publisher work")
			}
			if err := machine.DrainAsync(&result); err != nil {
				t.Fatal(err)
			}
			wantCount := beforeAccounts
			if active {
				wantCount++
			}
			if got := len(machine.Org.Objects["Account"].Records); got != wantCount {
				t.Fatalf("subscriber records=%d, want %d", got, wantCount)
			}
			if active {
				found := false
				for _, row := range machine.Org.Objects["Account"].Records {
					found = found || row.Fields["Name"].String == "Original payload"
				}
				if !found {
					t.Fatal("subscriber lost its publication snapshot")
				}
			}
			if err := machine.DrainAsync(&result); err != nil {
				t.Fatal(err)
			}
			if got := len(machine.Org.Objects["Account"].Records); got != wantCount {
				t.Fatal("repeated drain redelivered Flow")
			}
			if got := len(machine.Org.Objects["AsyncApexJob"].Records); got != beforeJobs {
				t.Fatal("Flow delivery leaked an Apex job row")
			}
		})
	}
}
