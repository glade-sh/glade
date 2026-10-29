package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardEmailMessageInputNativeProjectRoutes(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/standard_emailmessage_input_fields/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Case, Route, Source, Message string
		ClassName                         string `json:"class_name"`
		SourceSHA                         string `json:"source_sha256"`
		Compiled, Success                 bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 {
		t.Fatalf("native route matrix=%d, want 30", len(rows))
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Case, func(t *testing.T) {
			digest := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(digest[:]) != row.SourceSHA {
				t.Fatal("native source changed")
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
				if row.API != "67.0" || !strings.Contains(row.Message, "FIELD_INTEGRITY_EXCEPTION") {
					t.Fatal("unexpected native DML carry")
				}
			}
			if row.Route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one executed test; Glade %+v", run.Summary())
				}
				if !row.Success {
					t.Logf("native typed DML carry: %s; local runner problem=%v", row.Message, run.Suites[0].Cases[0].Problem)
					return
				}
				if run.Suites[0].Cases[0].Problem != nil {
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
			_, err = runner.execute(program)
			if !row.Success {
				t.Logf("native typed DML carry: %s; local execution error=%v", row.Message, err)
				return
			}
			if err != nil {
				t.Fatalf("native success; Glade %s", strings.TrimSpace(err.Error()))
			}
		})
	}
}
