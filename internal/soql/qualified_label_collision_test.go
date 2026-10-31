package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The captured collision fixture distinguishes API values from master labels
// (sobject_json3_collision/org.tsv:2-4); qualification must not reverse them.
func TestQualifiedToLabelUsesMasterLabel(t *testing.T) {
	for _, tc := range []struct{ object, field string }{{"Account", "Industry"}, {"Opportunity", "StageName"}} {
		t.Run(tc.object, func(t *testing.T) {
			def := storage.ObjectDefinition{APIName: tc.object, Fields: map[string]storage.Field{tc.field: {APIName: tc.field, Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Agriculture", Label: "Banking"}, {Value: "Banking", Label: "Technology"}}}}}
			org := storage.OrgState{Objects: map[string]storage.ObjectState{tc.object: {Definition: def}}}
			for _, pair := range []struct{ value, want string }{{"Agriculture", "Banking"}, {"Banking", "Technology"}} {
				record := storage.Record{Object: tc.object, Fields: map[string]storage.Value{tc.field: storage.StringValue(pair.value)}}
				expr, ok := parseSelectFieldExpression("toLabel(" + tc.object + "." + tc.field + ")")
				if !ok {
					t.Fatal("parse expression")
				}
				got, ok := selectFieldExpressionValue(org, def, record, expr)
				if !ok || got.String != pair.want {
					t.Errorf("%s got %q present=%v want %q", pair.value, got.String, ok, pair.want)
				}
			}
		})
	}
}
