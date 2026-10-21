package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native rows: a global class may implement Site.UrlRewriter, and the
// interface is usable as a variable type, instanceof target and cast target.
func TestSiteURLRewriterNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/site_url_rewriter"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16 || strings.Join(rows[0], ",") != "source_api,case,observation" {
		t.Fatal("incomplete native matrix")
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		if want[row[0]] == nil {
			want[row[0]] = map[string]string{}
		}
		if _, duplicate := want[row[0]][row[1]]; duplicate {
			t.Fatalf("duplicate native cell %s/%s", row[0], row[1])
		}
		want[row[0]][row[1]] = row[2]
	}
	classes := map[string]string{}
	for _, name := range []string{"UrlRewriterProbe", "UrlRewriterProbeTest"} {
		classes[name] = read("project/force-app/main/default/classes/" + name + ".cls")
	}
	construct := read("construct/UrlRewriterConstruct.cls")
	// Glade supports API 62.0 through 67.0; the 68.0 rows are retained as native evidence only.
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			rejected := nativeBoundaryIndex(t, api, map[string]string{"UrlRewriterConstruct": construct}, nil)
			if analysis := sema.Analyze(rejected); !rejected.HasErrors() && !analysis.HasErrors() {
				t.Error("native rejects new Site.UrlRewriter() (Type cannot be constructed); Glade accepted it")
			}
			run := Run(index, Options{SelectedClasses: []string{"UrlRewriterProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v", problem)
			}
			_, observed, _ := strings.Cut(problem.Message, "OBS|")
			var got map[string]string
			if err := json.Unmarshal([]byte(observed), &got); err != nil || len(got) != 5 {
				t.Fatalf("invalid observation %q: %v", observed, err)
			}
			for id, expected := range want[api] {
				if got[id] != expected {
					t.Errorf("%s: native=%q; Glade=%q", id, expected, got[id])
				}
			}
		})
	}
}
