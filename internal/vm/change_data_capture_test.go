package vm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChangeDataCaptureDeliveryAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			rows := readChangeDataCaptureDeliveryNative(t, api, route)
			for _, row := range []struct{ id, name string }{
				{"C01", "disabled_explicit_deliver"},
				{"C02", "enabled_explicit_deliver"},
				{"C03", "enabled_start_stop"},
				{"C04", "enabled_no_dml"},
			} {
				t.Run(api+"/"+route+"/"+row.name, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Account")
					org.Objects["CaptureLog__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
						APIName: "CaptureLog__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
							"Name": {APIName: "Name", Type: storage.FieldString},
						},
					}, Records: map[storage.ID]storage.Record{}}
					machine := New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
					}
					if err := machine.RegisterClass(Class{Name: "ChangeObserver", APIVersion: api, StaticFields: map[string]Field{
						"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)},
					}}); err != nil {
						t.Fatal(err)
					}
					body, err := CompileAnonymousWithOptions(`ChangeObserver.hits += Trigger.new.size(); insert new CaptureLog__c(Name='cdc');`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{Name: "ChangeSubscriber", Object: "AccountChangeEvent", Timing: "after", Operation: "insert", APIVersion: api, Program: body}); err != nil {
						t.Fatal(err)
					}
					source := `Map<String,Object> observed=new Map<String,Object>{'exception'=>null}; try {`
					if row.id != "C01" {
						source += `Test.enableChangeDataCapture();`
					}
					if row.id == "C03" {
						source += `Test.startTest();`
					}
					if row.id != "C04" {
						source += `insert new Account(Name='synthetic change');`
					}
					source += `observed.put('before',[SELECT count() FROM CaptureLog__c WHERE Name='cdc']);`
					if row.id == "C03" {
						source += `Test.stopTest();`
					} else {
						source += `Test.getEventBus().deliver();`
					}
					source += `observed.put('after',[SELECT count() FROM CaptureLog__c WHERE Name='cdc']); observed.put('static',ChangeObserver.hits);`
					source += `} catch(Exception failure) {observed.put('exception',failure.getTypeName());} System.assert(false,'CDC_OBS|'+JSON.serialize(observed));`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					var fault *RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("observation did not complete: %v", err)
					}
					_, payload, ok := strings.Cut(fault.ExceptionMessage(), "CDC_OBS|")
					var got map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil {
						t.Fatalf("invalid observation: %s", fault.ExceptionMessage())
					}
					if !reflect.DeepEqual(got, rows[row.id]) {
						want, _ := json.Marshal(rows[row.id])
						t.Fatalf("%s native=%s; Glade=%s", row.id, want, payload)
					}
				})
			}
		}
	}
}

func readChangeDataCaptureDeliveryNative(t *testing.T, api, route string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "platform_transaction_paths", "cdc", "api"+strings.TrimSuffix(api, ".0"), "native", route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=" + route + "\n", "rows=4\n", "native_complete=true\n", "deployment_success=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
		t.Fatal("incomplete native CDC rows")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || cells[1] != "cdc" || cells[2] != route || cells[3] != "runtime" || rows[cells[0]] != nil {
			t.Fatalf("invalid native row: %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[4]), &value) != nil || value == nil {
			t.Fatal("invalid native JSON")
		}
		rows[cells[0]] = value
	}
	for _, id := range []string{"C01", "C02", "C03", "C04"} {
		if rows[id] == nil {
			t.Fatalf("missing native row %s", id)
		}
	}
	return rows
}
