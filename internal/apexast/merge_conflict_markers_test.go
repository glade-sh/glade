package apexast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// Salesforce rejects a class that still holds merge-conflict markers at
// source API 62.0 and 67.0 (testdata/merge_conflict_markers/org.tsv), so the
// parser must report a syntax error rather than accept a partial body.
func TestMergeConflictMarkersAreSyntaxErrors(t *testing.T) {
	dir := filepath.Join("testdata", "merge_conflict_markers")
	native, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		if !strings.Contains(string(native), api+"\tFailed\t4\t1\tExpecting '}' but was: '<'") {
			t.Fatalf("native rejection at %s is not recorded", api)
		}
	}
	source, err := os.ReadFile(filepath.Join(dir, "MarkerProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	file := ParseSource("MarkerProbe.cls", string(source))
	for _, diag := range file.Diagnostics {
		if diag.Severity == diagnostic.Error && diag.Code == "APEXPARSE001" {
			return
		}
	}
	t.Fatalf("merge-conflict markers parsed without a syntax error: %#v", file.Diagnostics)
}
