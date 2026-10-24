package automation

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// A direct automation call has no insert event or prior snapshot. Keep its
// unmodeled trace visible instead of claiming the field is unchanged.
func TestFlowEntryMissingPriorProjectRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_entry_missing_prior"
	for _, api := range []string{"62", "67"} {
		for _, phase := range []string{"Before", "After"} {
			for _, operand := range []string{"True", "False"} {
				t.Run(fmt.Sprintf("api%s/%sSave/direct_automation/%s", api, phase, operand), func(t *testing.T) {
					name := "Birch" + phase + operand
					path := filepath.Join(fixture, "api"+api, "force-app/main/default/flows", name+".flow-meta.xml")
					index, err := LoadProject(project.Project{FlowFiles: []string{path}})
					if err != nil || len(index.Diagnostics) != 0 || len(index.Flows) != 1 {
						t.Fatalf("load: %v %+v", err, index)
					}
					org := storage.NewOrgState()
					storage.EnsureStandardObject(&org, "Account")
					storage.EnsureStandardObject(&org, "Contact")
					ApplyToOrg(&org, index)
					row := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue(name), "Phone": storage.StringValue("Ready")}}
					if phase == "After" {
						state := org.Objects["Account"]
						state.Records[row.ID] = row.Clone()
						org.Objects["Account"] = state
					}
					engine := dml.NewEngine(&org)
					traces := 0
					engine.AutomationTracer = func(event string, args map[string]any) {
						if event != "apex.flow.rule" {
							return
						}
						traces++
						if args["modeled"] != false || args["matched"] != false {
							t.Errorf("missing-prior boundary: modeled=%v matched=%v, want false/false", args["modeled"], args["matched"])
						}
					}
					if phase == "Before" {
						err = engine.ApplyBeforeSaveFlows([]storage.Record{row})
					} else {
						_, err = engine.ApplyAutomation("Account", row.ID)
					}
					if err != nil {
						t.Fatal(err)
					}
					if traces != 1 {
						t.Fatalf("rule traces=%d, want 1", traces)
					}
					if len(org.Objects["Contact"].Records) != 0 {
						t.Fatal("missing-prior automation created a contact")
					}
				})
			}
		}
	}
}
