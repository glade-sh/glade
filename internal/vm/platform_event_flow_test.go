package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type platformEventFlowFixture struct {
	Captures []struct {
		SourceAPI, Context, TransportAPI, OrgIDHash, DeploymentID          string
		RawResponseSha256, ObserverSource, ObserverSourceSha256            string
		ObserverMetadata, ObserverMetadataSha256, AnswerJSON, AnswerSha256 string
	}
	MetadataFiles []struct{ Path, Source, Sha256 string }
}

type platformEventFlowCell struct {
	Case                         string          `json:"case"`
	Published                    json.RawMessage `json:"published"`
	CallerToken                  string          `json:"callerToken"`
	Names, Before, After, Repeat []string
}

func loadPlatformEventFlowFixture(t *testing.T) platformEventFlowFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/platform_event_flow_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture platformEventFlowFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestPlatformEventFlowNativeFixtureIntegrity(t *testing.T) {
	fixture := loadPlatformEventFlowFixture(t)
	digest := func(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }
	validHash := func(value string) bool { data, err := hex.DecodeString(value); return err == nil && len(data) == 32 }
	if len(fixture.Captures) != 3 {
		t.Fatal("native capture scope changed")
	}
	seen := map[string]bool{}
	for _, capture := range fixture.Captures {
		if seen[capture.SourceAPI] || (capture.SourceAPI != "52.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0") {
			t.Fatal("unexpected source API")
		}
		seen[capture.SourceAPI] = true
		if capture.Context != "istest" || capture.TransportAPI != "67.0" || capture.DeploymentID == "" || !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSha256) {
			t.Fatal("incomplete named native provenance")
		}
		for _, pair := range [][2]string{{capture.ObserverSource, capture.ObserverSourceSha256}, {capture.ObserverMetadata, capture.ObserverMetadataSha256}, {capture.AnswerJSON, capture.AnswerSha256}} {
			if digest(pair[0]) != pair[1] {
				t.Fatal("native source or answer digest changed")
			}
		}
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if err := xml.Unmarshal([]byte(capture.ObserverMetadata), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.API != capture.SourceAPI {
			t.Fatal("observer source pin changed")
		}
		var cells []platformEventFlowCell
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
			t.Fatal(err)
		}
		if len(cells) != 3 {
			t.Fatal("native cell scope changed")
		}
	}
	for _, file := range fixture.MetadataFiles {
		if digest(file.Source) != file.Sha256 {
			t.Fatalf("metadata digest changed: %s", file.Path)
		}
		if strings.HasSuffix(file.Path, ".flow-meta.xml") || strings.HasSuffix(file.Path, ".cls-meta.xml") {
			var metadata struct {
				API string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(file.Source), &metadata); err != nil {
				t.Fatal(err)
			}
			expected := "52.0"
			if strings.HasSuffix(file.Path, ".flow-meta.xml") {
				expected = "55.0"
			}
			if metadata.API != expected {
				t.Fatalf("fixture source pin changed: %s", file.Path)
			}
		}
	}
}

// The Flow stays at API55 and its invocable receiver at API52. These named
// observations assert explicit delivery effects, not subscriber order or batching.
func TestPlatformEventFlowNativeConformance(t *testing.T) {
	fixture := loadPlatformEventFlowFixture(t)
	for _, capture := range fixture.Captures {
		var cells []platformEventFlowCell
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
			t.Fatal(err)
		}
		for _, cell := range cells {
			t.Run(capture.SourceAPI+"/"+cell.Case, func(t *testing.T) {
				machine := platformEventFlowMachine(t, fixture, true)
				executePlatformEventFlow(t, machine, capture.SourceAPI, platformEventFlowBody(t, cell, "Eventbus"))
			})
		}
	}
}

func platformEventFlowMachine(t *testing.T, fixture platformEventFlowFixture, importFlow bool) *VM {
	t.Helper()
	org := testDataOrg()
	org.Objects["OwnedSignal__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "OwnedSignal__e", KeyPrefix: "e00", Metadata: map[string]string{"publishBehavior": "PublishAfterCommit"},
		Fields: map[string]storage.Field{"Token__c": {APIName: "Token__c", Type: storage.FieldString, Length: 80}},
	}, Records: map[storage.ID]storage.Record{}}
	var receiver string
	p := project.Project{Root: t.TempDir(), SourceAPIVersion: "67.0"}
	for _, file := range fixture.MetadataFiles {
		if file.Path == "classes/PlatformSignalRecorder.cls" {
			receiver = file.Source
		}
		if importFlow && strings.HasSuffix(file.Path, ".flow-meta.xml") {
			path := filepath.Join(p.Root, filepath.FromSlash(file.Path))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(file.Source), 0644); err != nil {
				t.Fatal(err)
			}
			p.FlowFiles = append(p.FlowFiles, path)
		}
	}
	if importFlow {
		index, err := automation.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(index.Diagnostics) != 0 {
			t.Fatalf("Flow import diagnostics: %+v", index.Diagnostics)
		}
		automation.ApplyToOrg(&org, index)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	start := strings.Index(receiver, "static void capture(")
	if start < 0 {
		t.Fatal("native receiver method missing")
	}
	start += strings.Index(receiver[start:], "{") + 1
	end, depth := start, 1
	for end < len(receiver) && depth > 0 {
		if receiver[end] == '{' {
			depth++
		}
		if receiver[end] == '}' {
			depth--
		}
		end++
	}
	if depth != 0 {
		t.Fatal("native receiver body incomplete")
	}
	body, err := CompileAnonymousWithOptions(receiver[start:end-1], CompileOptions{APIVersion: "52.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "PlatformSignalRecorder", APIVersion: "52.0", Methods: map[string]Method{
		"capture": {Name: "PlatformSignalRecorder.capture", ClassName: "PlatformSignalRecorder", ReturnType: "void", IsStatic: true, Access: "public", Modifiers: []string{"InvocableMethod"}, Params: []Param{{Name: "signals", Type: "List<OwnedSignal__e>"}}, APIVersion: "52.0", SourceContextBound: true, Program: body},
	}}); err != nil {
		t.Fatal(err)
	}
	return machine
}

func executePlatformEventFlow(t *testing.T, machine *VM, api, body string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "PlatformSignalProbe", APIVersion: api}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "PlatformSignalProbe.observe", ClassName: "PlatformSignalProbe", IsStatic: true, Access: "public", ReturnType: "void", APIVersion: api, SourceContextBound: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	invoke, err := CompileAnonymousWithOptions("PlatformSignalProbe.observe();", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(invoke); err != nil {
		t.Fatal(err)
	}
}

func platformEventFlowBody(t *testing.T, cell platformEventFlowCell, eventBus string) string {
	t.Helper()
	body := ""
	switch cell.Case {
	case "noEvents":
		body = "Test.getEventBus().deliver();" + platformEventFlowNames("none", cell.Names)
	case "oneEvent":
		var success bool
		if err := json.Unmarshal(cell.Published, &success); err != nil {
			t.Fatal(err)
		}
		body = "OwnedSignal__e signal=new OwnedSignal__e(Token__c='PFS-One');Database.SaveResult published=" + eventBus + ".publish(signal);"
		body += fmt.Sprintf("System.assertEquals(%t,published.isSuccess());", success)
		body += "System.assertEquals('" + cell.CallerToken + "',signal.Token__c);"
	case "mixedEvents":
		var successes []bool
		if err := json.Unmarshal(cell.Published, &successes); err != nil {
			t.Fatal(err)
		}
		body = "List<Database.SaveResult> published=" + eventBus + ".publish(new List<OwnedSignal__e>{new OwnedSignal__e(Token__c='PFS-Mixed'),new OwnedSignal__e()});"
		body += fmt.Sprintf("System.assertEquals(%d,published.size());", len(successes))
		for i, success := range successes {
			body += fmt.Sprintf("System.assertEquals(%t,published[%d].isSuccess());", success, i)
		}
	default:
		t.Fatalf("unobserved case %s", cell.Case)
	}
	if cell.Case != "noEvents" {
		body += platformEventFlowNames("before", cell.Before) + "Test.getEventBus().deliver();" + platformEventFlowNames("after", cell.After)
		body += "Test.getEventBus().deliver();" + platformEventFlowNames("repeated", cell.Repeat)
	}
	return body
}

func platformEventFlowNames(name string, expected []string) string {
	body := "List<Account> " + name + "=[SELECT Name FROM Account WHERE Name LIKE 'PFS-%' ORDER BY Name];"
	body += fmt.Sprintf("System.assertEquals(%d,%s.size());", len(expected), name)
	for i, value := range expected {
		body += fmt.Sprintf("System.assertEquals('%s',%s[%d].Name);", value, name, i)
	}
	return body
}

func platformEventFlowRule() storage.FlowRule {
	return storage.FlowRule{Name: "OwnedSignalCapture", APIVersion: "55.0", Active: true, SourceStatus: "Active", ProcessType: "AutoLaunchedFlow", TriggerType: "PlatformEvent", Steps: []storage.FlowStep{{Kind: "action", Action: platformEventFlowAction()}}}
}

func platformEventFlowAction() storage.FlowAction {
	return storage.FlowAction{Name: "CaptureSignal", ActionType: "apex", ActionName: "PlatformSignalRecorder", ClassName: "PlatformSignalRecorder", Inputs: []storage.WorkflowFieldUpdate{{Name: "signals", SourceField: "$Record"}}}
}

func TestPlatformEventFlowDeliveryWithoutImporter(t *testing.T) {
	fixture := loadPlatformEventFlowFixture(t)
	machine := platformEventFlowMachine(t, fixture, false)
	event := machine.Org.Objects["OwnedSignal__e"]
	event.Definition.FlowRules = []storage.FlowRule{platformEventFlowRule()}
	machine.Org.Objects["OwnedSignal__e"] = event
	executePlatformEventFlow(t, machine, "52.0", "EventBus.publish(new OwnedSignal__e(Token__c='PFS-One'));Test.getEventBus().deliver();"+platformEventFlowNames("outputs", []string{"PFS-One"}))
}

func TestPlatformEventFlowInactiveAndAbsentControls(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(fmt.Sprint("inactive=", inactive), func(t *testing.T) {
			machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
			if inactive {
				event := machine.Org.Objects["OwnedSignal__e"]
				rule := platformEventFlowRule()
				rule.Active = false
				rule.SourceStatus = "Draft"
				event.Definition.FlowRules = []storage.FlowRule{rule}
				machine.Org.Objects["OwnedSignal__e"] = event
			}
			executePlatformEventFlow(t, machine, "52.0", "EventBus.publish(new OwnedSignal__e(Token__c='PFS-One'));Test.getEventBus().deliver();"+platformEventFlowNames("outputs", nil))
		})
	}
}

func TestPlatformEventFlowDirectInputBinding(t *testing.T) {
	machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
	record := storage.Record{Object: "OwnedSignal__e", Fields: map[string]storage.Value{"Token__c": storage.StringValue("PFS-One")}}
	if err := machine.invokeFlowActionWithContext(platformEventFlowAction(), record, dml.FlowActionContext{}, &Result{}); err != nil {
		t.Fatal(err)
	}
	executePlatformEventFlow(t, machine, "52.0", platformEventFlowNames("outputs", []string{"PFS-One"}))
}

func TestPlatformEventFlowWrapperInputControl(t *testing.T) {
	machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
	record := storage.Record{Object: "OwnedSignal__e", Fields: map[string]storage.Value{"Token__c": storage.StringValue("PFS-One")}}
	context := dml.FlowActionContext{Records: map[string]storage.Record{"$record": record}}
	wrapper, err := machine.flowActionInputObject(platformEventFlowAction(), record, context, "SignalRequest", "signals", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	signal := wrapper.Fields["signals"]
	if wrapper.Type != "SignalRequest" || signal.Type != "OwnedSignal__e" || signal.Fields["Token__c"].Text != "PFS-One" {
		t.Fatalf("wrapper input changed: %#v", wrapper)
	}
}

// Apex identifier case folding applies to the observed Eventbus spelling as well
// as the canonical spelling used by the no-subscriber controls.
func TestPlatformEventFlowPublishCaseInsensitive(t *testing.T) {
	machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), false)
	executePlatformEventFlow(t, machine, "52.0", "Database.SaveResult published=Eventbus.publish(new OwnedSignal__e(Token__c='PFS-One'));System.assertEquals(true,published.isSuccess());Test.getEventBus().deliver();"+platformEventFlowNames("outputs", nil))
}

// The existing queue scheduler also calls this subscriber callback at stop
// time. Queueing here exercises that callback without asserting event ordering.
func TestPlatformEventFlowStopTimeSubscriberCallback(t *testing.T) {
	machine := platformEventFlowMachine(t, loadPlatformEventFlowFixture(t), true)
	machine.testContext.PlatformEvents = []storage.Record{{Object: "OwnedSignal__e", Fields: map[string]storage.Value{"Token__c": storage.StringValue("PFS-One")}}}
	if err := machine.drainTestPlatformEventsFrom(&Result{}, 0, true); err != nil {
		t.Fatal(err)
	}
	executePlatformEventFlow(t, machine, "52.0", platformEventFlowNames("outputs", []string{"PFS-One"}))
}
