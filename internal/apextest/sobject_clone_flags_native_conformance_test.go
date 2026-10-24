package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestSObjectCloneFlagsNativeProjectRoutes(t *testing.T) {
	fixture := filepath.Join("..", "vm", "testdata", "conformance", "sobject_clone_flags")
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, entry := range []string{"database", "keyword"} {
				t.Run("api"+api+"/"+route+"/"+entry, func(t *testing.T) {
					raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+api, route, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					reader := csv.NewReader(strings.NewReader(string(raw)))
					reader.Comma = '\t'
					rows, err := reader.ReadAll()
					if err != nil {
						t.Fatal(err)
					}
					if len(rows) != 74 {
						t.Fatalf("native rows=%d, want 73", len(rows)-1)
					}
					var body strings.Builder
					body.WriteString("List<String> observed=CloneFlagsObservation.observe();\n")
					fmt.Fprintf(&body, "System.assertEquals(%d,observed.size());\n", len(rows)-1)
					for i, row := range rows[1:] {
						expected := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\t", "\\t").Replace(strings.Join(row, "\t"))
						fmt.Fprintf(&body, "System.assertEquals('%s',observed[%d],'%s');\n", expected, i, row[0])
					}
					var checker strings.Builder
					checker.WriteString("private static void checkCapturedRow(List<String> rows) { List<String> expected = new List<String>{")
					for i, row := range rows[1:] {
						if i > 0 {
							checker.WriteString(",")
						}
						checker.WriteString("'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\t", "\\t").Replace(strings.Join(row, "\t")) + "'")
					}
					checker.WriteString("}; Integer last=rows.size()-1; System.assertEquals(expected[last],rows[last]); }")
					root := t.TempDir()
					err = filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, fileEntry fs.DirEntry, walkErr error) error {
						if walkErr != nil {
							return walkErr
						}
						if fileEntry.IsDir() {
							return nil
						}
						rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
						if err != nil {
							return err
						}
						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						source := string(data)
						if strings.HasSuffix(path, "/CloneFlagsObservation.cls") {
							if entry == "keyword" {
								start := strings.Index(source, "        Database.SaveResult result;")
								end := strings.Index(source[start:], "    private static String saveUser(") + start
								source = source[:start] + `        if (updateRecord) update record; else insert record;
        return 'success';
    }
` + source[end:]
							}
							source = regexp.MustCompile(`(?s)rows\.add\(.*?\);`).ReplaceAllString(source, "$0 checkCapturedRow(rows);")
							end := strings.LastIndex(source, "}")
							source = source[:end] + checker.String() + source[end:]
						}
						if strings.HasSuffix(path, "-meta.xml") {
							source = strings.ReplaceAll(source, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+".0</apiVersion>")
						}
						if rel == "sfdx-project.json" {
							source = strings.ReplaceAll(source, "67.0", api+".0")
						}
						writeFile(t, filepath.Join(root, rel), source)
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					if route == "istest" {
						writeFile(t, filepath.Join(root, "force-app/main/default/classes/CloneFlagsObservationTest.cls"), "@IsTest private class CloneFlagsObservationTest { @IsTest static void cloneFlagsAndJSONInsertRoute() {"+body.String()+"} }")
					}
					index := loadTestIndex(t, root)
					if route == "istest" {
						run := Run(index, Options{RuntimeRESTAPIVersion: "67.0", SelectedClasses: []string{"CloneFlagsObservationTest"}, NoDiskCache: true, Parallelism: 1})
						if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
							data, _ := json.Marshal(run)
							t.Fatalf("native project route: %s", data)
						}
					} else {
						org := orgFromIndex(index)
						org.APIVersion = api + ".0"
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(body.String(), vm.CompileOptions{APIVersion: api + ".0"})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
