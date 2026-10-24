package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheckReportsNativeDuplicateSourcesAsWarnings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "project", "testdata", "source_collisions", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 || strings.Join(records[0], "\t") != "case_id\tapi_version\tproject_dir\tclass_name\tdeploy_status\tclass_count\tdeployed_marker" {
		t.Fatalf("unexpected native case table header or rows: %d", len(records))
	}

	for _, record := range records[1:] {
		if len(record) != 7 {
			t.Fatalf("native case row has %d fields, want 7", len(record))
		}
		caseID, projectDir, className := record[0], record[2], record[3]
		if record[4] != "Succeeded" || record[5] != "1" || record[6] == "" {
			t.Fatalf("native case %s does not record one successfully deployed class", caseID)
		}
		t.Run(caseID, func(t *testing.T) {
			root := filepath.Join("..", "project", "testdata", "source_collisions", projectDir)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--no-cache"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0 for a warning; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
			}
			out := stdout.String()
			packageDirs := []string{"current", "legacy"}
			if strings.Contains(className, "62") || strings.Contains(className, "67") {
				packageDirs = []string{"alpha", "zulu"}
			}
			for _, want := range []string{
				`"code": "GLADEPROJECT001"`,
				`"severity": "warning"`,
				"duplicate Apex class",
				className,
				"project.packageDirs",
				"select one directory",
				"selected " + filepath.ToSlash(filepath.Join("packages", record[6], "main", "default", "classes", className+".cls")),
			} {
				if !strings.Contains(out, want) {
					t.Fatalf("warning output is missing %q", want)
				}
			}
			for _, packageDir := range packageDirs {
				want := filepath.ToSlash(filepath.Join("packages", packageDir, "main", "default", "classes", className+".cls"))
				if !strings.Contains(out, want) {
					t.Fatalf("stdout = %s, missing source path %q", out, want)
				}
			}
		})
	}
}

// Route twins: anonymous execution and the named Apex test runner consume the
// same composite project; expectations come from the captured deploy rows.
func TestNativeDuplicateClassWinnerExecutionRoutes(t *testing.T) {
	fixtureRoot := filepath.Join("..", "project", "testdata", "source_collisions")
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range records[1:] {
		for _, route := range []string{"exec_anonymous", "test_named"} {
			t.Run(row[0]+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(filepath.Join(fixtureRoot, row[2]))); err != nil {
					t.Fatal(err)
				}
				assertion := fmt.Sprintf("System.assertEquals('%s', %s.sourceMarker());", row[6], row[3])
				args := []string{"exec", "--project", root, "--json", assertion}
				if route == "test_named" {
					classPath := filepath.Join(root, "packages", row[6], "main", "default", "classes", "SourceSelectionTest.cls")
					writeTestFile(t, classPath, "@IsTest private class SourceSelectionTest { @IsTest static void nativeWinner() { "+assertion+" } }")
					writeTestFile(t, classPath+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, row[1]))
					args = []string{"test", "--project", root, "--class", "SourceSelectionTest", "--no-cache", "--no-serve", "--no-progress", "--parallelism", "1", "--json"}
				}
				var stdout, stderr bytes.Buffer
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
					t.Fatalf("native winner %s: exit=%d stderr=%s stdout=%s", row[6], code, stderr.String(), stdout.String())
				}
				if route == "test_named" {
					run, err := decodeTestRunJSON(stdout.Bytes())
					if err != nil {
						t.Fatal(err)
					}
					summary := run.Summary()
					if summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("named route did not execute exactly one passing case: %+v", summary)
					}
				}
			})
		}
	}
}
