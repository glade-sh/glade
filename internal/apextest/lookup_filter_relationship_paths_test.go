package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture holds native Salesforce outcomes for lookup filters whose
// criteria traverse relationships (record type names, person-account flags and
// a two-hop Contact.Account path). The Apex test asserts every outcome.
func TestLookupFilterRelationshipPathsMatchNativeRows(t *testing.T) {
	const fixture = "../../testdata/conformance/lookup_filter_relationship_paths"
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil || len(rows) != 75 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	class, err := os.ReadFile(filepath.Join(fixture, "LookupFilterPathsProbeTest.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		var expected []string
		for _, row := range rows[1:] {
			if row[0] == api {
				expected = append(expected, strings.Join(row[1:4], "|"))
			}
		}
		if len(expected) != 37 {
			t.Fatalf("native %s rows = %d", api, len(expected))
		}
		for _, row := range expected {
			if !strings.Contains(string(class), row) {
				t.Fatalf("test class does not pin native row %q", row)
			}
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), readFixtureFile(t, filepath.Join(fixture, "config/project-scratch-def.json")))
			err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				rel, relErr := filepath.Rel(fixture, path)
				if relErr != nil {
					return relErr
				}
				writeFile(t, filepath.Join(root, rel), readFixtureFile(t, path))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"LookupFilterPathsProbeTest.cls", "LookupFilterPathsProbeTest.cls-meta.xml"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name), strings.ReplaceAll(readFixtureFile(t, filepath.Join(fixture, name)), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
			}
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || len(run.ExcludedClasses) != 0 {
				var problems []string
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							problems = append(problems, fmt.Sprintf("%s: %s", c.MethodName, c.Problem.Message))
						}
					}
				}
				t.Fatalf("lookup filter paths: summary=%+v excluded=%+v problems=%v", got, run.ExcludedClasses, problems)
			}
		})
	}
}

func readFixtureFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
