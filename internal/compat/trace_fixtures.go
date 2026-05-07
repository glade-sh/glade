package compat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/vm"
)

type PostParityTraceFixture struct {
	Target   string                         `json:"target"`
	Surfaces []PostParityTraceSurfaceExpect `json:"surfaces"`
}

type PostParityTraceSurfaceExpect struct {
	Name           string                      `json:"name"`
	ExpectedEvents []PostParityTraceEventMatch `json:"expectedEvents"`
}

type PostParityTraceEventMatch struct {
	Name     string            `json:"name,omitempty"`
	Category string            `json:"category,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
}

type PostParityTraceReport struct {
	Target   string                         `json:"target"`
	Ready    bool                           `json:"ready"`
	Baseline string                         `json:"baseline"`
	Surfaces []PostParityTraceSurfaceResult `json:"surfaces"`
	Failures []string                       `json:"failures,omitempty"`
}

type PostParityTraceSurfaceResult struct {
	Name   string   `json:"name"`
	Ready  bool     `json:"ready"`
	Events []string `json:"events"`
}

func CheckPostParityTraceFixture(path string) (PostParityTraceReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PostParityTraceReport{}, err
	}
	var fixture PostParityTraceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return PostParityTraceReport{}, err
	}
	absPath, _ := filepath.Abs(path)
	report := PostParityTraceReport{
		Target:   fixture.Target,
		Ready:    true,
		Baseline: absPath,
	}
	for _, surface := range fixture.Surfaces {
		events, err := postParityTraceEvents(surface.Name)
		if err != nil {
			report.Failures = append(report.Failures, fmt.Sprintf("%s: %v", surface.Name, err))
			report.Surfaces = append(report.Surfaces, PostParityTraceSurfaceResult{Name: surface.Name, Ready: false})
			continue
		}
		result := PostParityTraceSurfaceResult{Name: surface.Name, Events: stableTraceEventNames(events), Ready: true}
		for _, expected := range surface.ExpectedEvents {
			if !postParityTraceHas(events, expected) {
				result.Ready = false
				report.Failures = append(report.Failures, fmt.Sprintf("%s missing trace event %+v", surface.Name, expected))
			}
		}
		report.Surfaces = append(report.Surfaces, result)
	}
	report.Ready = len(report.Failures) == 0
	if !report.Ready {
		return report, fmt.Errorf("post-parity trace fixture mismatch: %s", strings.Join(report.Failures, "; "))
	}
	return report, nil
}

func postParityTraceEvents(surface string) ([]trace.Event, error) {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "flow":
		return flowTraceEvents()
	case "visualforce":
		return visualforceTraceEvents()
	case "metadata-deploy":
		return metadataDeployTraceEvents()
	default:
		return nil, fmt.Errorf("unknown trace surface %q", surface)
	}
}

func flowTraceEvents() ([]trace.Event, error) {
	org := storage.NewOrgState()
	account := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Status__c": {APIName: "Status__c", Type: storage.FieldString},
			},
			FlowRules: []storage.FlowRule{{
				Name:    "TraceStatus",
				Active:  true,
				Formula: `Name = "Trace"`,
				FieldUpdates: []storage.WorkflowFieldUpdate{{
					Name:         "SetStatus",
					Field:        "Status__c",
					LiteralValue: "Flowed",
				}},
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureStandardObjectFields(&account.Definition)
	org.Objects["Account"] = account

	engine := dml.NewEngine(&org)
	var events []trace.Event
	engine.AutomationTracer = func(name string, args map[string]any) {
		events = append(events, trace.Instant(name, "apex.flow", int64(len(events)), args))
	}
	results := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Trace")},
	}})
	if len(results) != 1 || !results[0].Success {
		return nil, fmt.Errorf("flow insert failed: %#v", results)
	}
	return events, nil
}

func visualforceTraceEvents() ([]trace.Event, error) {
	program, err := vm.CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Trace saved'));
PageReference next = new PageReference('/apex/TraceDone');
next.setRedirect(true);
return next;
`)
	if err != nil {
		return nil, err
	}
	machine := vm.New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(vm.Class{
		Name: "TraceController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "TraceController.save",
				ClassName:  "TraceController",
				ReturnType: "PageReference",
				Program:    program,
			},
		},
	}); err != nil {
		return nil, err
	}
	result, err := machine.InvokeVisualforceAction("TraceController", "save", "/apex/TracePage", map[string]string{"mode": "fixture"})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("visualforce action failed: %#v", result.Error)
	}
	return result.Trace, nil
}

func metadataDeployTraceEvents() ([]trace.Event, error) {
	program, err := vm.CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomObject objectDef = new Metadata.CustomObject();
objectDef.fullName = 'Trace_Object__c';
objectDef.label = 'Trace Object';
objectDef.pluralLabel = 'Trace Objects';
container.addMetadata(objectDef);
Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
Metadata.DeployResult deployStatus = Metadata.Operations.checkDeployStatus(deploymentId, true);
System.assert(deployStatus.success);
System.assertEquals(1, deployStatus.numberComponentsDeployed);
`)
	if err != nil {
		return nil, err
	}
	machine := vm.New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		return nil, err
	}
	return result.Trace, nil
}

func postParityTraceHas(events []trace.Event, expected PostParityTraceEventMatch) bool {
	for _, event := range events {
		if expected.Name != "" && event.Name != expected.Name {
			continue
		}
		if expected.Category != "" && event.Category != expected.Category {
			continue
		}
		matchedArgs := true
		for key, want := range expected.Args {
			got, ok := event.Args[key]
			if !ok || fmt.Sprint(got) != want {
				matchedArgs = false
				break
			}
		}
		if matchedArgs {
			return true
		}
	}
	return false
}

func stableTraceEventNames(events []trace.Event) []string {
	seen := make(map[string]struct{})
	for _, event := range events {
		key := event.Name
		if event.Category != "" {
			key += " [" + event.Category + "]"
		}
		seen[key] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
