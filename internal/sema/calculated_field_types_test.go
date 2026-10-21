package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCalculatedFieldDescribeConformance(t *testing.T) {
	data, err := os.Open("../apextest/testdata/conformance/calculated_field_types/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	reader := csv.NewReader(data)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]string{
		"field.Account.Name": "String", "field.Account.PhotoUrl": "String",
		"field.Opportunity.ExpectedRevenue": "Decimal", "field.Opportunity.TotalOpportunityQuantity": "Decimal",
	}
	matched, carried := 0, 0
	for _, row := range rows[1:] {
		if len(row) != 5 || row[3] != "OBSERVED" {
			t.Fatalf("invalid native row: %q", row)
		}
		if row[0] == "object.RegulatoryCodeViolation" || row[0] == "object.Quote" {
			if row[4] != "false" {
				t.Fatalf("edition-context carry changed: %q", row)
			}
			carried++
			continue
		}
		want, ok := wantTypes[row[0]]
		if !ok {
			continue
		}
		var native struct {
			Present bool   `json:"present"`
			Type    string `json:"type"`
		}
		if err := json.Unmarshal([]byte(row[4]), &native); err != nil || !native.Present {
			t.Fatalf("invalid native field observation %q: %v", row, err)
		}
		parts := strings.Split(row[0], ".")
		definition, ok := storage.StandardObjectDefinition(parts[1])
		if !ok {
			t.Fatalf("missing standard object %s", parts[1])
		}
		field := definition.Fields[parts[2]]
		if !strings.EqualFold(field.DisplayType, native.Type) {
			t.Fatalf("%s display type %s differs from native %s", row[0], field.DisplayType, native.Type)
		}
		if got := semaApexTypeForStorageField(field); got != want {
			t.Fatalf("%s API%s/%s value type = %s, want %s", row[0], row[1], row[2], got, want)
		}
		matched++
	}
	if matched != 24 || carried != 12 {
		t.Fatalf("native cells: matched %d, edition carries %d; want 24 and 12", matched, carried)
	}
}

func TestCalculatedStandardFieldReadsUseDescribeValueType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "CalculatedFieldProbe.cls")
	writeSemaFile(t, path, `
public class CalculatedFieldProbe {
    public static void inspect(RegulatoryCodeViolation violation, Quote quote) {
        Integer days = violation.DaysOpen;
        Integer lines = quote.LineItemCount;
        acceptInteger(violation.DaysOpen);
        acceptInteger(quote.LineItemCount);
    }
    private static void acceptInteger(Integer value) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("calculated integer field reads should use their captured value type: %#v", result.Diagnostics)
	}
	for _, objectField := range []struct{ object, field string }{
		{"RegulatoryCodeViolation", "DaysOpen"},
		{"Quote", "LineItemCount"},
	} {
		definition, ok := storage.StandardObjectDefinition(objectField.object)
		if !ok {
			t.Fatalf("missing standard object %s", objectField.object)
		}
		field := definition.Fields[objectField.field]
		if field.Type != storage.FieldCalculated || field.DisplayType != "INTEGER" {
			t.Fatalf("%s.%s should retain calculated storage metadata: %#v", objectField.object, objectField.field, field)
		}
		if field.Createable == nil || *field.Createable || field.Updateable == nil || *field.Updateable {
			t.Fatalf("%s.%s should retain read-only metadata: %#v", objectField.object, objectField.field, field)
		}
	}
}

func TestCalculatedStorageFieldTypeUsesDisplayMetadata(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		field storage.Field
		want  string
	}{
		{"calculated integer", storage.Field{APIName: "Computed__c", Type: storage.FieldCalculated, DisplayType: "INTEGER"}, "Integer"},
		{"ordinary integer", storage.Field{APIName: "Ordinary__c", Type: storage.FieldInteger}, "Integer"},
		{"ordinary boolean", storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean}, "Boolean"},
		{"unknown calculated display", storage.Field{APIName: "Computed__c", Type: storage.FieldCalculated, DisplayType: "UNRECOGNIZED"}, ""},
		{"omitted calculated display", storage.Field{APIName: "Computed__c", Type: storage.FieldCalculated}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := semaApexTypeForStorageField(tc.field); got != tc.want {
				t.Fatalf("calculated field value type = %q, want %q", got, tc.want)
			}
		})
	}
}
