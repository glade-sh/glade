package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// TestCorpusShapeUnresolvedManagedType restores the original ShapeOpaque shape:
// an absent managed-namespace type in a parameter, return type and local.
// Salesforce rejects it with "Invalid type: cloudkit.Envelope"
// (testdata/corpus_shape/native/org.tsv:2,7), so it cannot live in the
// deployable project. Glade must report the same invalid type at the same
// boundary, and the unrelated project tests must still run. Ruling 49 closes
// the parameter and return positions too (namespace_external_positions), so
// Glade reports one diagnostic per position; native compilation stops at the
// first. Pin the exact Glade code and multiplicity for every position.
func TestCorpusShapeUnresolvedManagedType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("API"+api, func(t *testing.T) {
			root := copyCorpusShapeProject(t, api)
			source, err := os.ReadFile(filepath.Join(root, "native/rejected/ShapeOpaque.cls.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(source), "cloudkit.Envelope forward(cloudkit.Envelope request)") {
				t.Fatal("rejected shape lost its unresolved managed type")
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ShapeOpaque.cls"), string(source))
			index := loadTestIndex(t, root)
			if len(index.Diagnostics) != 0 {
				t.Fatalf("index diagnostics: %v", index.Diagnostics)
			}
			diagnostics := sema.Analyze(index).Diagnostics
			want := map[string]int{"GLADESEMA002": 1, "GLADESEMA004": 1, "GLADESEMA006": 1}
			if len(diagnostics) != 3 {
				t.Fatalf("want exactly three position diagnostics, got %v", diagnostics)
			}
			for _, d := range diagnostics {
				if want[d.Code] == 0 || d.Message != "Invalid type: cloudkit.Envelope" || !strings.HasSuffix(d.File, "ShapeOpaque.cls") {
					t.Fatalf("unexpected diagnostic: %+v", d)
				}
				want[d.Code]--
			}
			// The invalid class is excluded; independent project tests still run.
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if s := run.Summary(); s.Total != 14 || s.Passed != 14 || s.CompileErrors != 0 || s.ExcludedClasses != 1 {
				t.Fatalf("project run with invalid opaque class: %+v; %s", s, firstRunProblem(run))
			}
		})
	}
}
