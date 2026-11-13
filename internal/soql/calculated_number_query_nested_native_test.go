package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE testdata/calculated_number_query/nested.tsv:2-7: nested null
// consumers and parent relationship projections, anonymous and named tests.
func TestCalculatedNumberQueryNestedNative(t *testing.T) {
	org := operationOrg()
	obj := org.Objects["Pf__c"]
	formulas := []struct{ name, formula string }{
		{"Nested", "IF(ISBLANK(ABS(Num__c - 1)), 9, 1)"},
		{"NestedValue", "BLANKVALUE(ABS(Num__c - 1), 7)"},
		{"NestedText", "LEN(TEXT(ABS(Num__c - 1)))"},
		{"NestedCompare", "IF(ABS(Num__c - 1) > 0, 1, 2)"},
	}
	var fields []string
	for _, f := range formulas {
		name := f.name + "__c"
		fields = append(fields, name)
		obj.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldCalculated, DisplayType: "DOUBLE", Formula: f.formula, FormulaTreatBlanksAs: "BlankAsBlank", Scale: 2, ScaleSpecified: true}
	}
	obj.Definition.Fields["Parent__c"] = storage.Field{APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Pf__c"}, RelationshipName: "Parent__r"}
	obj.Definition.Relations = append(obj.Definition.Relations, storage.Relationship{Field: "Parent__c", ParentObjects: []string{"Pf__c"}, ParentRelationship: "Parent__r", ChildRelationship: "Children__r"})
	var parent storage.ID
	for id, rec := range obj.Records {
		if rec.Fields["Name"].String == "a_null" {
			parent = id
			break
		}
	}
	child := storage.Record{ID: "a01999999999999", Object: "Pf__c", Fields: map[string]storage.Value{"Name": storage.StringValue("child"), "Parent__c": storage.IDValue(parent)}}
	obj.Records[child.ID] = child
	org.Objects["Pf__c"] = obj
	direct, err := ParseAndExecute(org, "SELECT "+strings.Join(fields, ",")+" FROM Pf__c WHERE Name = 'a_null'")
	if err != nil {
		t.Fatal(err)
	}
	relFields := make([]string, len(fields))
	for i, f := range fields {
		relFields[i] = "Parent__r." + f
	}
	related, err := ParseAndExecute(org, "SELECT "+strings.Join(relFields, ",")+" FROM Pf__c WHERE Name = 'child'")
	if err != nil {
		t.Fatal(err)
	}
	if len(direct.Records) != 1 || len(related.Records) != 1 {
		t.Fatalf("direct=%d relationship=%d", len(direct.Records), len(related.Records))
	}
	for _, row := range readOperationTSV(t, "nested.tsv", "api\troute\tnested\tnestedValue\tnestedText\tnestedCompare", 6) {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			for i, f := range fields {
				if got := observedNumberText(direct.Records[0].Fields[f]); got != row[i+2] {
					t.Errorf("%s=%s want %s", f, got, row[i+2])
				}
				if got := observedNumberText(related.Records[0].Fields[relFields[i]]); got != row[i+2] {
					t.Errorf("%s=%s want %s", relFields[i], got, row[i+2])
				}
			}
		})
	}
}
