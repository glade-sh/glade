package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native permset_in_test residual rows (residuals.tsv): the shapes a test
// assignment reaches beyond the direct reads in org.tsv. Upsert and insert of a
// self assignment, nested and plain Version runAs, Queueable, future and Batch
// jobs, a Flow $Permission formula, UserRecordAccess, Approval.lock, a mixed
// list insert and a 5000-row assignment insert. Each class pair under
// residuals/api62 and residuals/api67 is the capture source for that API. The
// observation is the assertion text of one captured method; the timing row
// masks its milliseconds. Visualforce rendering is forbidden in a test, so the
// rendered $Permission value was not observed.
func TestPermissionSetResidualsNativeConformance(t *testing.T) {
	family := filepath.Join("testdata/conformance/permset_in_test")
	raw, err := os.Open(filepath.Join(family, "residuals.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+78 || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	indexes := map[string]*residualProject{}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		project := filepath.Join(family, "residuals", "api"+strings.TrimSuffix(api, ".0"))
		err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(project, path)
			writeFile(t, filepath.Join(root, rel), string(body))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		indexes[api] = &residualProject{index: loadTestIndex(t, root)}
	}
	msNative := regexp.MustCompile(`ms=\d+`)
	psaRowsNative := regexp.MustCompile(`psaRows=\d+`)
	whitespace := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	for i, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		line := i + 2
		t.Run(api+"/"+method, func(t *testing.T) {
			class, name, _ := strings.Cut(method, ".")
			limitMode := vm.LimitModeStrict
			if class == "PsrFinishPerfTest" {
				// The timing row only measures a 5000-row insert. The strict
				// per-statement heap walk costs minutes over its nested loop (it
				// is quadratic in the list the loop builds), so the row runs
				// without it; the row's observation does not depend on heap
				// accounting.
				limitMode = vm.LimitModePermissive
			}
			run := Run(indexes[api].index, Options{SelectedClasses: []string{class}, SelectedMethod: name, NoDiskCache: true, Parallelism: 1, LimitMode: limitMode})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			got := strings.TrimSpace(whitespace.Replace(strings.TrimPrefix(message, "System.AssertException: ")))
			got = strings.TrimPrefix(got, "Assertion Failed: ")
			got = msNative.ReplaceAllString(got, "ms=<ms>")
			if strings.HasPrefix(class, "Mx_") {
				// The assignment count is the scratch org's own baseline: the
				// API 67.0 org held two more assignments for the running user.
				got = psaRowsNative.ReplaceAllString(got, "psaRows=<n>")
				want = psaRowsNative.ReplaceAllString(want, "psaRows=<n>")
			}
			if got != strings.TrimSpace(want) {
				t.Fatalf("NATIVE %s permset_in_test/residuals.tsv:%d istest %s\nnative=%q\nglade =%q", method, line, api, want, got)
			}
		})
	}
}

type residualProject struct {
	index typesys.Index
}
