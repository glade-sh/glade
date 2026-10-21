package vm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestBatchNativeFaultProjectionPhases(t *testing.T) {
	// Owned native API62/67/68 captures distinguish stopTest's immediate
	// phase snapshot from anonymous jobs' eventual terminal state. Their
	// Iterable<SObject> returns one unsaved Account and executeBatch uses 1.
	for _, named := range []bool{false, true} {
		for _, optIn := range []bool{false, true} {
			for _, phase := range []string{"START", "EXECUTE", "FINISH", "NONE"} {
				if !optIn && phase != "EXECUTE" {
					continue // The captured opt-out control throws only in execute.
				}
				t.Run(fmt.Sprintf("named=%t/optIn=%t/%s", named, optIn, phase), func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					org.APIVersion = "67.0"
					account := org.Objects["Account"]
					account.Records = make(map[storage.ID]storage.Record)
					account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
					org.Objects["Account"] = account
					machine.SetOrg(&org)
					if named {
						machine.EnableTestContext()
					}
					compile := func(source string) ir.Program {
						t.Helper()
						program, err := CompileAnonymous(source)
						if err != nil {
							t.Fatal(err)
						}
						return program
					}
					bodies := map[string]string{
						"START":  "return new List<SObject>{new Account(Name='scope')};",
						"FINISH": "insert new Account(Name='finish witness');",
					}
					if phase != "NONE" {
						bodies[phase] = "throw new IllegalArgumentException('synthetic " + strings.ToLower(phase) + "');"
					}
					interfaces := []string{"Database.Batchable<SObject>"}
					if optIn {
						interfaces = append(interfaces, "Database.RaisesPlatformEvents")
					}
					if err := machine.RegisterClass(Class{
						Name: "FaultWave", APIVersion: "67.0", Interfaces: interfaces,
						Methods: map[string]Method{
							"start": {Name: "FaultWave.start", ClassName: "FaultWave", ReturnType: "Iterable<SObject>",
								Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: compile(bodies["START"])},
							"execute": {Name: "FaultWave.execute", ClassName: "FaultWave", ReturnType: "void",
								Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: compile(bodies["EXECUTE"])},
							"finish": {Name: "FaultWave.finish", ClassName: "FaultWave", ReturnType: "void",
								Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: compile(bodies["FINISH"])},
						},
					}); err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{
						Name: "FaultWitness", Object: "BatchApexErrorEvent", Timing: triggerTimingAfter, Operation: "insert",
						Program: compile(`for (BatchApexErrorEvent fault : Trigger.new) {
 insert new Account(Name='event witness', Description=JSON.serialize(new Map<String,Object>{
  'phase'=>fault.Phase, 'type'=>fault.ExceptionType, 'message'=>fault.Message, 'scope'=>fault.JobScope,
  'job'=>String.valueOf(fault.AsyncApexJobId), 'jobLength'=>String.valueOf(fault.AsyncApexJobId).length()
 }));
}`),
					}); err != nil {
						t.Fatal(err)
					}
					job := AsyncJob{ID: "707000000000001", Kind: "BatchApex", Object: Object("FaultWave"), BatchSize: 1}
					jobs := []AsyncJob{job}
					draining, chained := false, false
					err := machine.drainAsyncJobs(&Result{}, &jobs, &draining, &chained)
					if (err != nil) != (named && phase != "NONE") {
						t.Fatalf("drain error = %v", err)
					}
					if err != nil {
						kind, message, _ := machine.batchErrorExceptionDetails(err)
						if kind != "System.IllegalArgumentException" || message != "synthetic "+strings.ToLower(phase) {
							t.Fatalf("stop exception = %s: %s", kind, message)
						}
					}
					status, total, processed, faults := "Completed", int64(1), int64(1), int64(0)
					if phase != "NONE" {
						if named {
							switch phase {
							case "START":
								status, total, processed = "Preparing", 0, 0
							case "EXECUTE":
								status, processed = "Processing", 0
							}
						} else {
							faults = 1
							if phase == "START" {
								status, total = "Failed", 0
							}
						}
					}
					row := machine.Org.Objects["AsyncApexJob"].Records[storage.ID(job.ID)]
					if row.Fields["Status"].String != status || row.Fields["TotalJobItems"].Integer != total ||
						row.Fields["JobItemsProcessed"].Integer != processed || row.Fields["NumberOfErrors"].Integer != faults {
						t.Fatalf("job projection = %v; want %s/%d/%d/%d", row.Fields, status, total, processed, faults)
					}
					finishCount, eventCount := 0, 0
					for _, row := range machine.Org.Objects["Account"].Records {
						switch row.Fields["Name"].String {
						case "finish witness":
							finishCount++
						case "event witness":
							eventCount++
							var payload map[string]any
							if err := json.Unmarshal([]byte(row.Fields["Description"].String), &payload); err != nil {
								t.Fatal(err)
							}
							var wantScope any
							if phase == "EXECUTE" {
								wantScope = "null"
							}
							if payload["scope"] != wantScope || payload["phase"] != phase || payload["type"] != "System.IllegalArgumentException" || payload["message"] != "synthetic "+strings.ToLower(phase) {
								t.Fatalf("event payload = %v", payload)
							}
							if payload["job"] != job.ID || payload["jobLength"] != float64(15) {
								t.Fatalf("event job identity = %v/%v, want %s/15", payload["job"], payload["jobLength"], job.ID)
							}
						}
					}
					wantFinish, wantEvents := 0, 0
					if phase == "NONE" || (!named && phase == "EXECUTE") {
						wantFinish = 1
					}
					if optIn && phase != "NONE" {
						wantEvents = 1
					}
					if finishCount != wantFinish || eventCount != wantEvents {
						t.Fatalf("finish/event witnesses = %d/%d, want %d/%d", finishCount, eventCount, wantFinish, wantEvents)
					}
				})
			}
		}
	}
}

func TestBatchNativeFaultProjectionBounds(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "63.0", "66.0", "67.0", "68.0", "69.0"} {
		for _, size := range []int{1, 200} {
			t.Run(fmt.Sprintf("%s/size%d", api, size), func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{
					Name: "FaultWave", APIVersion: api, Interfaces: []string{"Database.Batchable<SObject>"},
					Methods: map[string]Method{"start": {Name: "FaultWave.start", ClassName: "FaultWave", ReturnType: "Iterable<SObject>"}},
				}); err != nil {
					t.Fatal(err)
				}
				job := AsyncJob{Kind: "BatchApex", Object: Object("FaultWave"), BatchSize: size}
				want := size == 1 && (api == "62.0" || api == "67.0" || api == "68.0")
				fault := newExceptionError("IllegalArgumentException", "synthetic fault")
				if !machine.capturedBatchFaultException(job, "EXECUTE", fault) || machine.capturedBatchFaultException(job, "START", fault) || machine.capturedBatchFaultException(job, "FINISH", fault) {
					t.Fatal("uncaptured opt-out start/finish fault acquired native projection")
				}
				if got := machine.capturedBatchFaultJob(job); got != want {
					t.Fatalf("captured job = %t, want %t", got, want)
				}
				if !machine.capturedBatchFaultScope([]Value{Object("Account")}) || machine.capturedBatchFaultScope(nil) ||
					machine.capturedBatchFaultScope([]Value{Object("Account"), Object("Account")}) || machine.capturedBatchFaultScope([]Value{Object("Contact")}) {
					t.Fatal("scope boundary did not preserve unsaved Account singleton restriction")
				}
				withID := Object("Account")
				withID.Fields["Id"] = String("001000000000001AAA")
				if machine.capturedBatchFaultScope([]Value{withID}) {
					t.Fatal("saved Account unexpectedly uses the captured unsaved scope projection")
				}
			})
		}
	}
}

func TestBatchNativeFaultStackKeepsInternalCallsAndDiagnostics(t *testing.T) {
	frames := []callFrame{
		{Symbol: "Entry.capture", File: "entry.cls", Line: 3, Column: 17},
		{Symbol: "FaultWave.execute", File: "wave.cls", Line: 11, Column: 29},
		{Symbol: "FaultWave.helper", File: "wave.cls", Line: 18, Column: 25},
		{Symbol: "FaultWave.helper", File: "wave.cls", Line: 20, Column: 13},
		{Symbol: "FaultWave.helper", File: "wave.cls", Line: 22, Column: 41, statement: true},
	}
	value := Object("IllegalArgumentException")
	value.Fields["message"] = String("synthetic execute")
	value = annotateException(value, frames)
	originalText := value.Fields["__stackTrace"]
	original := &apexThrowError{value: value, stack: frames}
	projected := projectBatchFaultExceptionStack(original, 1).(*apexThrowError)
	want := "Class.FaultWave.helper: line 22, column 1\nClass.FaultWave.helper: line 18, column 1\nClass.FaultWave.execute: line 11, column 1"
	if got := projected.value.Fields["__stackTrace"].Text; got != want {
		t.Fatalf("projected stack = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(projected.stack, frames) || !reflect.DeepEqual(original.stack, frames) || !reflect.DeepEqual(original.value.Fields["__stackTrace"], originalText) {
		t.Fatal("text projection mutated original exception or structured diagnostic frames")
	}
	if !reflect.DeepEqual(projected.value.Fields["__lineNumber"], value.Fields["__lineNumber"]) || exceptionMessage(projected.value) != "synthetic execute" {
		t.Fatal("text projection changed exception line or message")
	}
	if projectBatchFaultExceptionStack(original, 0) != original || projectBatchFaultExceptionStack(original, len(frames)) != original {
		t.Fatal("invalid or absent parent boundary unexpectedly changed the exception")
	}
}
