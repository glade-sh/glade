package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// org.tsv binds the constructor/query body to native API62/67 anonymous,
// named and @IsTest observations. Metadata layouts are alternative inputs to
// the same project shape; without-feature rejections retain their own oracle.
func TestPersonAccountEvidenceProjectRoutes(t *testing.T) {
	const dir = "testdata/conformance/person_account_evidence"
	raw, err := os.ReadFile(filepath.Join(dir, "evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var shapes []struct {
		Name    string
		Enabled bool
		Files   map[string]string
	}
	if err := json.Unmarshal(raw, &shapes); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "observe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 11 {
		t.Fatalf("native rows: %v, %d", err, len(rows))
	}
	for _, row := range rows[1:] {
		if row[2] == "constructor-query" && (row[3] != "PASS" || row[4] != fmt.Sprintf("%x", sha256.Sum256(raw))) {
			t.Fatal("native body or result changed")
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range shapes {
			for _, route := range []string{"check", "anonymous", "named", "test-runner"} {
				t.Run(api+"/"+shape.Name+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					// Keep the retrieved project's explicit Account schema in every route.
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
					for path, source := range shape.Files {
						writeFile(t, filepath.Join(root, path), source)
					}
					const class = "PersonEvidenceProbe"
					source := "public class " + class + " { public static void sample(){" + body + "} }"
					if route == "test-runner" {
						source = "@IsTest private class " + class + " { @IsTest static void sample(){" + body + "} }"
					}
					if route != "anonymous" {
						path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, root)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					analysis := sema.Analyze(index)
					if route == "anonymous" {
						analysis = sema.AnalyzeAnonymous(index, body, api)
					}
					if analysis.HasErrors() == shape.Enabled {
						t.Fatalf("enabled=%t; compilation diagnostics=%v", shape.Enabled, analysis.Diagnostics)
					}
					if !shape.Enabled {
						var messages []string
						for _, d := range analysis.Diagnostics {
							messages = append(messages, d.Message+" "+d.NativeMessage)
						}
						if !strings.Contains(strings.Join(messages, " "), "FirstName") {
							t.Fatalf("expected native field rejection, got %v", analysis.Diagnostics)
						}
						if route == "test-runner" {
							run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
							if run.Summary().OK() {
								t.Fatalf("missing runner compile rejection: %+v", run.Summary())
							}
						}
						return
					}
					if route == "check" {
						return
					}
					if route == "test-runner" {
						for _, state := range []string{"cold", "reused"} {
							t.Run(state, func(t *testing.T) {
								run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
								if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
									t.Fatalf("runner: %+v %+v", run.Summary(), run.Suites)
								}
							})
						}
						return
					}
					programSource := body
					if route == "named" {
						programSource = class + ".sample();"
					}
					program, err := vm.CompileAnonymousWithOptions(programSource, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
