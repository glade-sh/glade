package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// TestStripOmittedDefaultsNativeConformance replays native
// Security.stripInaccessible and absent custom metadata observations
// (org.tsv, ORG-RUN.txt). Each probe method reports one line per subject, and
// every line must match its row. carried.tsv names each line where Glade still
// differs from its row, with the exact Glade line and the reason. A carried line
// that changes or starts to match fails the test.
func TestStripOmittedDefaultsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "strip_omitted_defaults")
	rows := readStripOmittedDefaultsTSV(t, filepath.Join(dir, "org.tsv"), "source_api,class,method,subject,observation")
	type group struct {
		api, class, method string
		lines              []string
	}
	var groups []*group
	byKey := map[string]*group{}
	for _, row := range rows {
		key := row[0] + "/" + row[1] + "/" + row[2]
		current := byKey[key]
		if current == nil {
			current = &group{api: row[0], class: row[1], method: row[2]}
			byKey[key] = current
			groups = append(groups, current)
		}
		current.lines = append(current.lines, row[3]+" "+row[4])
	}
	carries := map[string]string{}
	for _, row := range readStripOmittedDefaultsTSV(t, filepath.Join(dir, "carried.tsv"), "source_api,class,method,subject,glade_observation,reason") {
		if row[5] == "" {
			t.Fatalf("carry without a reason: %v", row)
		}
		carries[row[0]+"/"+row[1]+"/"+row[2]+"/"+row[3]] = row[4]
	}
	index := loadTestIndex(t, filepath.Join(dir, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, current := range groups {
		t.Run(fmt.Sprintf("%s/%s/%s", current.class, current.api, current.method), func(t *testing.T) {
			class := current.class + strings.TrimSuffix(current.api, ".0")
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: current.method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "\n")
			if !found {
				t.Fatalf("invalid observation: %s", result.Problem.Message)
			}
			got := strings.Split(strings.TrimSpace(payload), "\n")
			if len(got) != len(current.lines) {
				t.Fatalf("native:\n%s\nGlade:\n%s", strings.Join(current.lines, "\n"), strings.Join(got, "\n"))
			}
			for i, want := range current.lines {
				subject, _, _ := strings.Cut(want, " ")
				carry, carried := carries[current.api+"/"+current.class+"/"+current.method+"/"+subject]
				switch {
				case got[i] == want && carried:
					t.Errorf("line now matches native; remove its carry: %s", want)
				case got[i] != want && !carried:
					t.Errorf("native=%s; Glade=%s", want, got[i])
				case got[i] != want && got[i] != subject+" "+carry:
					t.Errorf("carried line changed: native=%s; carried=%s; Glade=%s", want, carry, got[i])
				}
			}
		})
	}
}

func readStripOmittedDefaultsTSV(t *testing.T, path, header string) [][]string {
	t.Helper()
	raw, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != header ||
		(header == "source_api,class,method,subject,observation" && len(rows) < 2) {
		t.Fatalf("incomplete rows in %s", path)
	}
	return rows[1:]
}
