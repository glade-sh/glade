package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The checked-in control has complete metadata; it does not assert the default
// of any unavailable managed-package field.
func TestR14CascadeNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/r14cascade"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], "\t") != "source_api\troute\tdefault\texplicit\tnull_negation" {
		t.Fatalf("invalid native rows: %v", err)
	}
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), fmt.Sprintf("anonymous_sha256=%x\n", sha256.Sum256(anonymous))) {
		t.Fatal("anonymous source differs from capture")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || row[2] != "false" || row[3] != "true" || row[4] != "System.NullPointerException" || seen[row[0]+row[1]] {
			t.Fatal("invalid or duplicate native observation")
		}
		api, route := row[0], row[1]
		seen[api+route] = true
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(fixture, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				name = filepath.ToSlash(name)
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256(raw))) {
					t.Fatalf("source mismatch: %s", name)
				}
				rendered := strings.ReplaceAll(string(raw), "62.0", api)
				if !strings.Contains(string(capture), fmt.Sprintf("source_api_sha256:%s:%s=%x\n", api, name, sha256.Sum256([]byte(rendered)))) {
					t.Fatalf("capture input mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), rendered)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if api != "62.0" {
				// admin_field_grant_sources (2026-11-05): from API 67 an administrator
				// reads or writes an ordinary custom field only through an assigned
				// permission set or a profile grant. The native R14 cascade row passed at 67.0,
				// so its capture user held the fixture's R14Access set; replay that grant.
				writeCapturedAssignments(t, root, "R14Access")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			if len(index.Triggers) != 1 {
				t.Fatalf("expected captured trigger, got %d", len(index.Triggers))
			}
			if route == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{"R14DriverTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("native pass, Glade: %+v", summary)
				}
				return
			}
			if analysis := sema.AnalyzeAnonymous(index, string(anonymous), api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			result, err := runner.execute(program)
			if err != nil || len(result.Debug) != 1 || result.Debug[0] != "R14_CAPTURE_PASS" {
				t.Fatalf("native pass; Glade: %v %v", result.Debug, err)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatal("missing captured route")
	}
}
