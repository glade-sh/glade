package vm

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestChangeEventPartialUpdatePriorIdentityEngineRoute(t *testing.T) {
	rows, err := os.ReadFile("../apextest/testdata/conformance/flow_delivery_paths/org.tsv")
	if err != nil || !strings.Contains(string(rows), "\tcdcPartial\t") || !strings.Contains(string(rows), "changed=Description/nulled=/diff=") {
		t.Fatalf("missing native partial-save/header observations: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, invalidFirst := range []bool{true, false} {
			name := api + "/valid_first"
			if invalidFirst {
				name = api + "/invalid_first"
			}
			t.Run(name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				engine := dml.NewEngine(&org)
				engine.Now = func() time.Time { return time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC) }
				seed := storage.Record{Object: "Account", Fields: map[string]storage.Value{
					"Name": storage.StringValue("Cedar surviving row"), "Description": storage.StringValue("before"),
				}}
				saved := engine.Insert([]storage.Record{seed})
				if len(saved) != 1 || !saved[0].Success {
					t.Fatalf("seed: %+v", saved)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				machine.Triggers["AccountChangeEvent"] = []Trigger{{Name: "CedarObserver", Object: "AccountChangeEvent", Timing: triggerTimingAfter, Operation: "insert"}}
				engine.ChangeObserver = machine.observeChangeDataCapture
				valid := storage.Record{Object: "Account", ID: saved[0].ID, Fields: map[string]storage.Value{"Description": storage.StringValue("after")}}
				invalid := storage.Record{Object: "Account", ID: "001000000000099", Fields: map[string]storage.Value{"Description": storage.StringValue("unwritten")}}
				inputs := []storage.Record{valid, invalid}
				if invalidFirst {
					inputs = []storage.Record{invalid, valid}
				}
				results := engine.Update(inputs)
				successes := 0
				for _, result := range results {
					if result.Success {
						successes++
					}
				}
				if successes != 1 {
					t.Fatalf("partial update: %+v", results)
				}
				events := machine.materializeChangeDataCapture(machine.testContext.changeJournal)
				if len(events) != 1 {
					t.Fatalf("surviving events=%d; want 1", len(events))
				}
				fields := events[0].Fields
				if fields["Name"].Kind != storage.ValueNull || fields["Description"].String != "after" {
					t.Fatalf("UPDATE projection lost surviving prior identity: %+v", fields)
				}
				if want := storage.ListValue(storage.StringValue("Description")); !storage.ValuesEqual(fields["ChangeEventHeader.changedFields"], want) {
					t.Fatalf("changed fields=%+v; want %+v", fields["ChangeEventHeader.changedFields"], want)
				}
			})
		}
	}
}
