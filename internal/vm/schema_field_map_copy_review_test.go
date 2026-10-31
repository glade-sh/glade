package vm_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Review cases C08-C47 are native rows for both routes and API versions.
func TestSchemaFieldMapCopyReviewConformance(t *testing.T) {
	const dir = "../../testdata/conformance/schema_field_map_copy"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]string{}
	for _, row := range rows[1:] {
		observed[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	debug := regexp.MustCompile(`System\.debug\('ROAD123\|(C[0-9]+)\|'\+(.+)\);`)
	for _, api := range []string{"62.0", "67.0"} {
		var named strings.Builder
		named.WriteString("@IsTest private class FieldMapCopyReview {\n")
		for _, name := range []string{"capture", "detail", "lower", "keys"} {
			raw, err := os.ReadFile(filepath.Join(dir, "review-"+name+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			source := debug.ReplaceAllStringFunc(string(raw), func(line string) string {
				parts := debug.FindStringSubmatch(line)
				id := parts[1]
				want := observed[api+"/anonymous/"+id]
				namedWant := observed[api+"/isTest/"+id]
				if (want != "true" && want != "false") || namedWant != want {
					t.Fatalf("missing or conflicting oracle %s %s", api, id)
				}
				count++
				return fmt.Sprintf("System.assertEquals(%s,%s,'%s');", want, parts[2], id)
			})
			if count == 0 {
				t.Fatalf("no cases in %s", name)
			}
			fmt.Fprintf(&named, "@IsTest static void observed%s(){\n%s\n}\n", name, source)
			t.Run(api+"/anonymous/"+name, func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
		named.WriteString("}\n")
		root := t.TempDir()
		path := filepath.Join(root, "FieldMapCopyReview.cls")
		if err := os.WriteFile(path, []byte(named.String()), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0600); err != nil {
			t.Fatal(err)
		}
		index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: api}, schema.Schema{})
		run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
		count := 0
		for _, suite := range run.Suites {
			for _, result := range suite.Cases {
				count++
				if result.Status != testreport.StatusPass {
					t.Errorf("%s @IsTest %s: %v", api, result.MethodName, result.Problem)
				}
			}
		}
		if count != 4 {
			t.Errorf("%s @IsTest: got %d methods, want 4", api, count)
		}
	}
}
