package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardConstructorCompleteNativeProjectRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_constructor_complete"
	raw, err := os.ReadFile(fixture + "/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Object, Case, Route, Source, Observation string
		ClassName                                     string `json:"class_name"`
		SourceSHA                                     string `json:"source_sha256"`
		Compiled, Success                             bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fixture + "/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	native, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 54 || len(native) != 55 {
		t.Fatalf("native matrix JSON=%d TSV=%d", len(rows), len(native)-1)
	}
	for i, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Object+"/"+row.Case, func(t *testing.T) {
			digest := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(digest[:]) != row.SourceSHA || native[i+1][7] != row.SourceSHA || native[i+1][6] != strings.ReplaceAll(strings.ReplaceAll(row.Observation, "\n", " "), "\r", " ") {
				t.Fatal("native source or observation changed")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if row.Route != "anonymous" {
				p := filepath.Join(root, "force-app/main/default/classes", row.ClassName+".cls")
				writeFile(t, p, row.Source)
				writeFile(t, p+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			if got := !index.HasErrors() && !analysis.HasErrors(); got != row.Compiled {
				t.Fatalf("native compiled=%t; Glade=%t: %v %v", row.Compiled, got, index.Diagnostics, analysis.Diagnostics)
			}
			if !row.Compiled {
				return
			}
			if !row.Success {
				t.Fatal("unhandled native failure")
			}
			if row.Route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native pass; Glade %+v", run.Summary())
				}
				return
			}
			source := row.Source
			if row.Route == "named" {
				source = row.ClassName + ".sample();"
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if row.Case == "missing-put" && !strings.Contains(strings.Join(result.Debug, "\n"), "OBSERVED|"+row.Observation) {
				t.Fatalf("native %q; debug=%v", row.Observation, result.Debug)
			}
		})
	}
}
