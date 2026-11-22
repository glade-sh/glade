package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestFlowPricebookVisibilityControlsNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "flow_pricebook_visibility_controls")
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			root := filepath.Join(fixture, "api"+strings.TrimSuffix(row[1], ".0"))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("fixture diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			className := "BookSeeAllTest"
			if row[0] == "custom" {
				className = "BookCustomTest"
			} else if row[0] != "all_data" {
				t.Fatalf("unknown case %q", row[0])
			}
			result := Run(index, Options{SelectedClasses: []string{className}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
			if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %#v", result.Summary())
			}
			problem := result.Suites[0].Cases[0].Problem
			if problem == nil || flowRefsExceptionType(problem.Type) != "System.AssertException" {
				t.Fatalf("expected observation, got %#v", problem)
			}
			_, want, ok := strings.Cut(row[5], "OBS=")
			if !ok {
				t.Fatalf("missing native observation: %q", row[5])
			}
			_, got, ok := strings.Cut(problem.Message, "OBS=")
			if !ok || got != want {
				t.Fatalf("native %q, Glade %q", want, problem.Message)
			}
		})
	}
}
