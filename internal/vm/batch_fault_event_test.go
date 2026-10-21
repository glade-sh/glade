package vm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestBatchFaultEventPhaseTransport(t *testing.T) {
	// The public BatchApexErrorEvent reference specifies the opt-in marker,
	// three phases and exception fields. Exact trace text and hosted/test
	// delivery timing are deliberately not asserted here.
	// https://developer.salesforce.com/docs/platform/platform-events/guide/sforce-api-objects-batchapexerrorevent.html
	for _, phase := range []string{"START", "EXECUTE", "FINISH", "NONE"} {
		for _, optIn := range []bool{true, false} {
			name := phase + "/optOut"
			if optIn {
				name = phase + "/optIn"
			}
			t.Run(name, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				account := org.Objects["Account"]
				account.Records = make(map[storage.ID]storage.Record)
				account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
				org.Objects["Account"] = account
				machine.SetOrg(&org)
				compile := func(code string) ir.Program {
					t.Helper()
					program, err := CompileAnonymous(code)
					if err != nil {
						t.Fatal(err)
					}
					return program
				}
				bodies := map[string]string{
					"START": "return new List<SObject>{new Account(Name='scope')};",
				}
				if phase != "NONE" {
					bodies[phase] = "throw new IllegalArgumentException('synthetic " + strings.ToLower(phase) + "');"
				}
				interfaces := []string{"Database.Batchable<SObject>"}
				if optIn {
					interfaces = append(interfaces, "Database.RaisesPlatformEvents")
				}
				if err := machine.RegisterClass(Class{
					Name: "FaultWave", Interfaces: interfaces,
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
 insert new Account(Name=fault.Phase, Description=JSON.serialize(new Map<String,Object>{
  'type'=>fault.ExceptionType,'message'=>fault.Message,'stack'=>fault.StackTrace
 }));
}`),
				}); err != nil {
					t.Fatal(err)
				}
				err := machine.runBatchJob(AsyncJob{ID: "707000000000001", Object: Object("FaultWave"), BatchSize: 200}, &Result{})
				if (err != nil) != (phase != "NONE") {
					t.Fatalf("batch error = %v, phase = %s", err, phase)
				}
				rows := machine.Org.Objects["Account"].Records
				want := 0
				if optIn && phase != "NONE" {
					want = 1
				}
				if len(rows) != want {
					t.Fatalf("subscriber rows = %d, want %d (batch error %v)", len(rows), want, err)
				}
				for _, row := range rows {
					var payload map[string]string
					if err := json.Unmarshal([]byte(row.Fields["Description"].String), &payload); err != nil {
						t.Fatal(err)
					}
					if row.Fields["Name"].String != phase || payload["type"] != "System.IllegalArgumentException" ||
						payload["message"] != "synthetic "+strings.ToLower(phase) || !strings.Contains(payload["stack"], "FaultWave.") {
						t.Fatalf("incorrect fault transport: phase=%v payload=%v", row.Fields["Name"], payload)
					}
				}
			})
		}
	}
}
