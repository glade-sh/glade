package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The selected Contact channel delivers without enablement; the separate
// unconfigured Account control remains in the original delivery route test.
func TestChangeDataCaptureSelectedEntityAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				row := readChangeDataCaptureExtendedRow(t, api, route, "CDC03")
				org := storage.NewOrgState()
				org.APIVersion = api + ".0"
				storage.EnsureStandardObject(&org, "Contact")
				if err := json.Unmarshal([]byte(`{"changeEventEntities":["ContactChangeEvent"]}`), &org.Metadata); err != nil {
					t.Fatal(err)
				}
				machine := changeDataCaptureCounterMachine(t, &org, "ContactChangeEvent")
				if route == "istest" {
					machine.EnableTestContext()
				}
				expected := "null"
				count := 0
				if exception, ok := row["exception"].(string); ok {
					expected = "'" + exception + "'"
				}
				if events, ok := row["events"].([]any); ok {
					count = len(events)
				}
				source := fmt.Sprintf(`String exceptionType; try { insert new Contact(LastName='selected'); Test.getEventBus().deliver(); } catch(Exception failure) {exceptionType=failure.getTypeName();} System.assertEquals(%s,exceptionType); System.assertEquals(%d,CaptureCounter.hits);`, expected, count)
				executeChangeDataCaptureSource(t, machine, api, source)
			})
		}
	}
}

func TestChangeDataCaptureSetupReplayRuntimeCloneAPIRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"snapshot", "journal"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				row := readChangeDataCaptureExtendedRow(t, api, "istest", "CDC17")
				org := storage.NewOrgState()
				org.APIVersion = api + ".0"
				storage.EnsureStandardObject(&org, "Account")
				setup := changeDataCaptureCounterMachine(t, &org, "AccountChangeEvent")
				setup.EnableTestContext()
				executeChangeDataCaptureSource(t, setup, api, "insert new Account(Name='oracle-setup');")
				capture, ok := any(setup).(interface{ CaptureChangeDataCaptureTestSetup() })
				if !ok {
					t.Fatal("setup-to-method route has no CDC snapshot")
				}
				capture.CaptureChangeDataCaptureTestSetup()
				for method := 0; method < 2; method++ { // each method replays setup: cdc_g1_g3/org.tsv:10-11,34-35
					copy := org.CloneRuntime()
					machine := changeDataCaptureCounterMachine(t, &copy, "AccountChangeEvent")
					if route == "journal" {
						machine.SetIsolationJournal(storage.NewIsolationJournal(&copy))
					}
					machine.EnableTestContext()
					executeChangeDataCaptureSource(t, machine, api, fmt.Sprintf(`System.assertEquals(%v,[SELECT count() FROM Account WHERE Name='oracle-setup']); Test.enableChangeDataCapture(); Test.getEventBus().deliver(); System.assertEquals(%d,CaptureCounter.hits); Test.getEventBus().deliver(); System.assertEquals(%d,CaptureCounter.hits);`, row["setupRecords"], len(row["events"].([]any)), len(row["events"].([]any))))
				}
			})
		}
	}
}

func changeDataCaptureCounterMachine(t *testing.T, org *storage.OrgState, event string) *VM {
	t.Helper()
	machine := New(nil)
	machine.SetOrg(org)
	if err := machine.RegisterClass(Class{Name: "CaptureCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
		t.Fatal(err)
	}
	trigger, err := CompileAnonymousWithOptions(`CaptureCounter.hits += Trigger.new.size();`, CompileOptions{APIVersion: org.APIVersion})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "CaptureSubscriber", Object: event, Timing: "after", Operation: "insert", APIVersion: org.APIVersion, Program: trigger}); err != nil {
		t.Fatal(err)
	}
	return machine
}

func executeChangeDataCaptureSource(t *testing.T, machine *VM, api, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api + ".0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func readChangeDataCaptureExtendedRow(t *testing.T, api, route, id string) map[string]any {
	t.Helper()
	dir := filepath.Join("..", "apextest/testdata/conformance/cdc_journal_delivery/extended/cdc/native", "api"+api, route)
	note, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil || !strings.Contains(string(note), "native_complete=true") {
		t.Fatalf("incomplete native capture: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) == 5 && cells[0] == id {
			var row map[string]any
			if err := json.Unmarshal([]byte(cells[4]), &row); err != nil {
				t.Fatal(err)
			}
			return row
		}
	}
	t.Fatalf("missing %s native row", id)
	return nil
}
