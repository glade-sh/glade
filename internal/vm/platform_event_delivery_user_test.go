package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPlatformEventSubscriberUserRoutesNativeRows(t *testing.T) {
	testPlatformEventSubscriberUserRoutes(t, true)
}

func TestPlatformEventApexOnlySubscriberUserRoutesNativeRows(t *testing.T) {
	testPlatformEventSubscriberUserRoutes(t, false)
}

func testPlatformEventSubscriberUserRoutes(t *testing.T, withFlow bool) {
	t.Helper()
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/platform_event_flow_elements/native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range []string{"anonymous", "deliver", "stop"} {
			var want map[string]any
			for _, line := range strings.Split(string(raw), "\n") {
				cells := strings.SplitN(line, "\t", 3)
				if len(cells) == 3 && cells[0] == "PE07" && cells[1] == route {
					if err := json.Unmarshal([]byte(cells[2]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want == nil {
				t.Fatalf("missing native subscriber users: %s/%s", api, route)
			}
			for _, throws := range []bool{false, true} {
				name := api + "/" + route + "/success"
				if throws {
					name = api + "/" + route + "/error_restoration"
				}
				t.Run(name, func(t *testing.T) {
					machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
					storage.EnsureDeterministicPlatformData(machine.Org)
					if route == "anonymous" {
						machine.testContext = nil
					}
					machine.SetCurrentUser(storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("Standard")}})
					caller := machine.executionUser
					var testUser Value
					if machine.testContext != nil {
						machine.testContext.CurrentUser = caller
						testUser = machine.testContext.CurrentUser
					}
					flowUser := "Standard"
					if want["flowAutomated"] == true {
						flowUser = "AutomatedProcess"
					}
					triggerUser := "Standard"
					if want["triggerAutomated"] == true {
						triggerUser = "AutomatedProcess"
					}
					triggerBody := "System.assertEquals('" + triggerUser + "',UserInfo.getUserType());"
					if throws && !withFlow {
						triggerBody += "throw new IllegalArgumentException('owned subscriber error');"
					}
					trigger, err := CompileAnonymousWithOptions(triggerBody, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{Name: "SignalUser", Object: "OwnedSignal__e", Timing: "after", Operation: "insert", APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
					body := "System.assertEquals('" + flowUser + "',UserInfo.getUserType());"
					if throws && withFlow {
						body += "throw new IllegalArgumentException('owned subscriber error');"
					}
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "DeliveryUserAction", APIVersion: api, Methods: map[string]Method{"capture": {Name: "DeliveryUserAction.capture", ClassName: "DeliveryUserAction", ReturnType: "void", IsStatic: true, Access: "public", Modifiers: []string{"InvocableMethod"}, Params: []Param{{Name: "signals", Type: "List<OwnedSignal__e>"}}, APIVersion: api, SourceContextBound: true, Program: program}}}); err != nil {
						t.Fatal(err)
					}
					event := machine.Org.Objects["OwnedSignal__e"]
					if withFlow {
						rule := platformEventFlowRule()
						rule.Steps[0].Action.ClassName = "DeliveryUserAction"
						rule.Steps[0].Action.ActionName = "DeliveryUserAction"
						event.Definition.FlowRules = []storage.FlowRule{rule}
					}
					machine.Org.Objects["OwnedSignal__e"] = event
					records := []storage.Record{{Object: "OwnedSignal__e", Fields: map[string]storage.Value{"Token__c": storage.StringValue("unit")}}}
					if route == "anonymous" {
						err = machine.deliverPlatformEventSubscribers(records, &Result{})
					} else {
						machine.testContext.PlatformEvents = records
						err = machine.drainTestPlatformEventsFrom(&Result{}, 0, route == "stop")
					}
					if throws && (withFlow || route != "stop") {
						if err == nil || !strings.Contains(err.Error(), "owned subscriber error") {
							t.Fatalf("subscriber error: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(machine.executionUser, caller) || machine.testContext != nil && !reflect.DeepEqual(machine.testContext.CurrentUser, testUser) {
						t.Fatal("subscriber changed caller user")
					}
				})
			}
		}
	}
}
