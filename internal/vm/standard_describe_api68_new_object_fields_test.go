package vm

import (
	"encoding/csv"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native API68 anonymous maps are recorded in P0-new-objects/org.tsv.
func TestStandardAPI68NewObjectFieldMaps(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/standard_describe_api68_drift/P0-new-objects/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("malformed native row: %v", row)
		}
		if row[0] != "68.0" || row[1] != "default" || row[2] != "anonymous" || row[5] != "true" {
			continue
		}
		if want[row[3]] == nil {
			want[row[3]] = map[string]bool{}
		}
		want[row[3]][strings.ToLower(row[4])] = true
	}
	if len(want) != 17 {
		t.Fatalf("native objects=%d, want 17", len(want))
	}
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	storage.EnsureStandardObject(&org, "AssociatedLocationHistory")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	for name, expected := range want {
		t.Run(name, func(t *testing.T) {
			value := fieldMapGet(t, machine, fieldMapAPI68Wrapper(t, machine, name))
			got := map[string]bool{}
			for _, token := range value.Map {
				got[strings.ToLower(token.Fields["field"].Text)] = true
			}
			if !maps.Equal(got, expected) {
				t.Fatalf("native fields=%v; Glade=%v", expected, got)
			}
		})
	}
}
