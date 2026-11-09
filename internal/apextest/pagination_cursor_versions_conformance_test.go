package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

const paginationCursorVersionsFixture = "testdata/conformance/pagination_cursor_versions"

// Native rows: pagination_cursor_versions org.tsv. Each PgCur<api>C<n> class
// (class API 61.0 to 67.0) deploys and its @IsTest method records the call's
// result. Every class compiles natively, so Glade must compile the project and
// replay each run row exactly.
func TestPaginationCursorVersionsNativeConformance(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(paginationCursorVersionsFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 43 || lines[0] != "api\tcall\tphase\tclass\toutcome" {
		t.Fatalf("incomplete pagination cursor rows: %d", len(lines))
	}
	root := t.TempDir()
	project := filepath.Join(paginationCursorVersionsFixture, "project")
	if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), string(content))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		api, call, phase, class, outcome := cells[0], cells[1], cells[2], cells[3], cells[4]
		t.Run(api+"/"+call+"/"+phase, func(t *testing.T) {
			if phase == "deploy" {
				if outcome != "Succeeded" {
					t.Fatalf("unexpected native deploy outcome %q", outcome)
				}
				for _, d := range append(append([]diagnostic.Diagnostic(nil), index.Diagnostics...), analysis.Diagnostics...) {
					if d.Severity == diagnostic.Error && strings.Contains(filepath.Base(d.File), class+".") {
						t.Fatalf("native compiles %s; Glade: %s", class, d.Message)
					}
				}
				return
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected the %s observation: %+v", class, run.Summary())
			}
			_, got, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "CAPTURE|")
			if !ok || strings.TrimSpace(got) != outcome {
				t.Fatalf("native %q; Glade %q", outcome, run.Suites[0].Cases[0].Problem.Message)
			}
		})
	}
}
