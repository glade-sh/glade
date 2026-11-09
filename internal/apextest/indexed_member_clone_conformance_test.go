package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// Native answers: testdata/conformance/indexed_member_clone/org.tsv. A member
// chain through an indexed List element (`holder.items[i].parts.clone()`) types
// as the field's List, and a same-named inner class in two top-level classes
// resolves per owner.
func TestIndexedMemberCloneNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/indexed_member_clone"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	for _, row := range rows[1:] {
		native[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	if len(native) != 8 {
		t.Fatalf("native rows=%d, want 8", len(native))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(raw), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			for _, key := range []string{"named/deploy", "named/indexed", "named/inherited"} {
				if native[api+"/"+key] == "" {
					t.Fatalf("missing native row %s", key)
				}
			}
			run := Run(index, Options{SelectedClasses: []string{"UseItTest"}, NoDiskCache: true, Parallelism: 1})
			if len(run.ExcludedClasses) != 0 || run.Summary().Total != 2 {
				t.Fatalf("named route: excluded=%v summary=%#v", run.ExcludedClasses, run.Summary())
			}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Status != testreport.StatusPass {
						t.Fatalf("%s: %#v", c.Name, c.Problem)
					}
				}
			}
			body, err := os.ReadFile(filepath.Join(fixture, "anon.apex"))
			if err != nil {
				t.Fatal(err)
			}
			if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
				t.Fatalf("anonymous: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatalf("anonymous run: %v", err)
			}
		})
	}
}
