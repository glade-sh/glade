package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowPriorTextCriteriaBeforeAndAfterSave(t *testing.T) {
	for _, route := range []string{"before_save", "after_save"} {
		for _, tc := range []struct {
			name, operation, operand string
			prior                    storage.Record
			want                     bool
		}{
			{name: "equal", operation: "equals", operand: "T1", prior: storage.Record{ID: "003000000000001", Fields: map[string]storage.Value{"Title": storage.StringValue("T1")}}, want: true},
			{name: "not_equal", operation: "notEqual", operand: "T1", prior: storage.Record{ID: "003000000000001", Fields: map[string]storage.Value{"Title": storage.StringValue("T2")}}, want: true},
			{name: "null_insert", operation: "isNull", operand: "true", want: true},
			{name: "not_null", operation: "isNull", operand: "false", prior: storage.Record{ID: "003000000000001", Fields: map[string]storage.Value{"Title": storage.StringValue("T1")}}, want: true},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"Title": {APIName: "Title", Type: storage.FieldString}}}
				frame := newFlowFrame()
				frame.saveDecisions, frame.priorDecisions = true, true
				frame.beforeSaveDecisions = route == "before_save"
				frame.records[flowFrameKey("$Record__Prior")] = tc.prior
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Record__Prior.Title", Operation: tc.operation, Value: tc.operand}}}
				matches, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, definition, frame)
				if !supported || matches != tc.want {
					t.Fatalf("matches=%t supported=%t; want %t", matches, supported, tc.want)
				}
			})
		}
	}
}
