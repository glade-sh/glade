package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardRecordTypeFieldPolicyNativeProjectRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_recordtype_field_policy"
	raw, err := os.ReadFile(filepath.Join(dir, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Profile, Route, Source, Message string
		ClassName                            string `json:"class_name"`
		SourceSHA                            string `json:"source_sha256"`
		Compiled                             bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "classes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var classes map[string]string
	if err := json.Unmarshal(raw, &classes); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	oracle, err := reader.ReadAll()
	if err != nil || len(rows) != 20 || len(oracle) != 21 {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	metadata, err := os.ReadFile(filepath.Join(dir, "recordtype-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		compiled := !strings.HasSuffix(row.Route, "constructor") || row.Compiled
		digest := sha256.Sum256([]byte(row.Source))
		if hex.EncodeToString(digest[:]) != row.SourceSHA || strings.Join(oracle[i+1], "\t") != strings.Join([]string{row.API, row.Profile, row.Route, strconv.FormatBool(compiled), row.Message, row.SourceSHA}, "\t") {
			t.Fatal("native source or oracle changed")
		}
		t.Run(row.API+"/"+row.Profile+"/"+row.Route, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), []byte(`{"edition":"Developer"}`), 0600); err != nil {
				t.Fatal(err)
			}

			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			api := strings.TrimSuffix(row.API, ".0")
			class := "RecordTypeFieldPolicy" + api
			selected := map[string]string{class: classes[class], class + "Test": classes[class+"Test"]}
			if row.Route == "named-constructor" {
				selected[row.ClassName] = row.Source
			}
			for name, source := range selected {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			}
			if row.Profile == "recordtype" {
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/recordTypes/OwnedPolicy.recordType-meta.xml"), string(metadata))
			}
			index := loadTestIndex(t, root)
			if strings.HasSuffix(row.Route, "constructor") {
				analysis := sema.Analyze(index)
				if row.Route == "anonymous-constructor" {
					analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
				}
				if got := !index.HasErrors() && !analysis.HasErrors(); got != row.Compiled {
					t.Fatalf("native compiled=%t (%s); Glade=%t: %v %v", row.Compiled, row.Message, got, index.Diagnostics, analysis.Diagnostics)
				}
				return
			}
			if row.Route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{class + "Test"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("missing project case: %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") || !strings.HasSuffix(problem.Message, "OBSERVED|"+row.Message) {
					t.Fatalf("native OBSERVED|%s; Glade %+v", row.Message, problem)
				}
				return
			}
			source := row.Source
			if row.Route == "named" {
				source = "System.assert(false,'OBSERVED|'+" + row.ClassName + ".observe());"
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			_, err = runner.execute(program)
			var observed *vm.RuntimeError
			if !errors.As(err, &observed) || observed.Type != "System.AssertException" || !strings.HasSuffix(observed.ExceptionMessage(), "OBSERVED|"+row.Message) {
				t.Fatalf("native OBSERVED|%s; Glade %v", row.Message, err)
			}
		})
	}
}
