package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowChainAuditQualifiedAndLoweredParentUpdateRoutes(t *testing.T) {
	for _, route := range []struct{ name, source string }{
		{"frame_qualified", "$Record.LastModifiedDate"},
		{"loader_lowered", "LastModifiedDate"},
		{"loader_lowered_casefold", "lastmodifieddate"},
	} {
		for _, kind := range []storage.FieldType{storage.FieldDate, storage.FieldDateTime} {
			t.Run(route.name+"/"+string(kind), func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				frame := newFlowFrame()
				frame.saveDecisions, frame.priorDecisions = true, true
				source := storage.Record{Object: "Contact", System: storage.SystemFields{LastModifiedDate: "2026-10-19T01:02:03.000Z"}}
				definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime}}}
				seedFlowFrame(&engine, &source, frame)
				// parent=true: the parent-relationship path the native
				// PARENT rows observe (see the precedence comment below).
				value, null, ok := engine.flowRecordUpdateAssignmentValue(storage.Field{Type: kind}, source, storage.WorkflowFieldUpdate{SourceField: route.source}, definition, frame, true)
				want := storage.DateTimeValue(source.System.LastModifiedDate)
				if kind == storage.FieldDate {
					want = storage.DateValue("2026-10-19")
				}
				if !ok || null || !storage.ValuesEqual(value, want) {
					t.Fatalf("value=%#v null=%t supported=%t; native=%#v", value, null, ok, want)
				}
			})
		}
	}
	for _, reference := range []string{"$Record.LastModifiedDate", "LastModifiedDate"} {
		for _, control := range []struct {
			name   string
			fields map[string]storage.Value
			nulls  map[string]bool
			want   storage.Value
		}{
			{name: "explicit_value", fields: map[string]storage.Value{"LastModifiedDate": storage.DateTimeValue("2026-10-18T00:00:00.000Z")}, want: storage.DateTimeValue("2026-10-18T00:00:00.000Z")},
			{name: "field_null", fields: map[string]storage.Value{"LastModifiedDate": storage.NullValue()}, want: storage.NullValue()},
			{name: "marker_null", nulls: map[string]bool{"LastModifiedDate": true}, want: storage.NullValue()},
		} {
			t.Run("precedence/"+reference+"/"+control.name, func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				frame := newFlowFrame()
				frame.saveDecisions = true
				source := storage.Record{Object: "Contact", Fields: control.fields, ExplicitNulls: control.nulls, System: storage.SystemFields{LastModifiedDate: "2026-10-19T01:02:03.000Z"}}
				definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime}}}
				seedFlowFrame(&engine, &source, frame)
				// Parent assignment resolves through the audit-enabled source
				// helper, the only caller the native rows observe:
				// internal/apextest/testdata/conformance/flow_decision_shapes/
				// org.tsv, API 62.0 and 67.0, anonymous and istest,
				// PARENT:Open and PARENT:Open3 -> dt=set|dd=gmt. The
				// explicit-value and explicit-null precedence assertions
				// are unchanged.
				got, ok := engine.flowRecordUpdateAuditSourceValue(reference, source, definition, frame)
				if !ok || !storage.ValuesEqual(got, control.want) {
					t.Fatalf("value=%#v supported=%t; explicit field=%#v", got, ok, control.want)
				}
			})
		}
	}
}
