package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowFollowupsDecisionExplicitRecordBindings(t *testing.T) {
	// Native FV rows in flow_followups/org.tsv distinguish these scalar defaults
	// from the record values, with rejection twins for each explicit operand.
	for _, tc := range []struct {
		name, recordName     string
		employees            int64
		wantName, wantNumber bool
	}{
		{"FV_name_number_collision", "FF_SHADOW_PASS", 19, true, true},
		{"FV_explicit_record_number_twin", "FF_SHADOW_PASS", 7, true, false},
		{"FV_explicit_record_name_twin", "FF_SHADOW_OTHER", 19, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			engine := NewEngine(&org)
			definition := org.Objects["Account"].Definition
			source := storage.Record{Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue(tc.recordName), "NumberOfEmployees": storage.IntegerValue(tc.employees),
			}}
			frame := newFlowFrame()
			frame.scalars[flowFrameKey("Name")] = storage.StringValue("FLOW_NAME")
			frame.scalars[flowFrameKey("NumberOfEmployees")] = storage.DecimalValue("7.00")
			for _, probe := range []struct {
				name, expression string
				want             bool
			}{
				{"Name", `AND(Name = "FLOW_NAME", $Record.Name = "FF_SHADOW_PASS")`, tc.wantName},
				{"NumberOfEmployees", `AND(NumberOfEmployees = 7, $Record.NumberOfEmployees = 19)`, tc.wantNumber},
				{"quoted_reference", `"$Record.Name" = "$Record.Name"`, true},
			} {
				t.Run(probe.name, func(t *testing.T) {
					got, supported := engine.evaluateFlowDecisionFormula(probe.expression, source, definition, frame)
					if !supported || got != probe.want {
						t.Fatalf("expression=%s got=%v supported=%v want=%v", probe.expression, got, supported, probe.want)
					}
				})
			}
		})
	}
}

func TestFlowFollowupsExplicitRecordPriorContext(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	engine := NewEngine(&org)
	definition := org.Objects["Account"].Definition
	source := storage.Record{Object: "Account", ID: "001000000000071", Fields: map[string]storage.Value{"NumberOfEmployees": storage.IntegerValue(19)}}
	prior := source.Clone()
	prior.Fields["NumberOfEmployees"] = storage.IntegerValue(11)
	engine.PriorRecords = map[storage.ID]storage.Record{source.ID: prior}
	frame := newFlowFrame()
	frame.scalars[flowFrameKey("NumberOfEmployees")] = storage.IntegerValue(7)
	// Explicit record qualification must preserve the parser's existing prior
	// context. Compare aliases rather than inventing a new native update row.
	for _, expression := range []string{`PRIORVALUE(%s) = 11`, `ISCHANGED(%s)`} {
		bare := strings.ReplaceAll(expression, "%s", "NumberOfEmployees")
		explicit := strings.ReplaceAll(expression, "%s", "$Record.NumberOfEmployees")
		want, bareOK := engine.evaluateFlowDecisionFormula(bare, source, definition, frame)
		got, explicitOK := engine.evaluateFlowDecisionFormula(explicit, source, definition, frame)
		if !bareOK || !explicitOK || got != want {
			t.Errorf("%s: explicit=%v/%v bare=%v/%v", expression, got, explicitOK, want, bareOK)
		}
	}
}
