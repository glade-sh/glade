package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestGeneratedSOQLRelationshipNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/generated_soql_relationships"
	capture, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatal("incomplete native capture")
	}

	caseTable, err := os.ReadFile(filepath.Join(directory, "case-table.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases := strings.Split(strings.TrimSpace(string(caseTable)), "\n")
	if len(cases) != 12 || cases[0] != "case_id\tbehavior\tsource_shape\tobservation_key\texpected_json" {
		t.Fatal("expected the captured relationship and safe-navigation case table")
	}

	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 3 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("expected native observations for both API versions")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		columns := strings.SplitN(row, "\t", 3)
		if len(columns) != 3 || (columns[0] != "62.0" && columns[0] != "67.0") || columns[1] != "istest" || seen[columns[0]] {
			t.Fatalf("invalid or duplicate native observation: %q", row)
		}
		api := columns[0]
		seen[api] = true
		t.Run(api+"/istest", func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(columns[2]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != len(cases)-1 {
				t.Fatalf("native observation has %d behaviors; case table has %d", len(want), len(cases)-1)
			}
			for _, line := range cases[1:] {
				fields := strings.SplitN(line, "\t", 5)
				if len(fields) != 5 {
					t.Fatalf("invalid case-table row: %q", line)
				}
				var expected any
				if err := json.Unmarshal([]byte(fields[4]), &expected); err != nil {
					t.Fatalf("invalid expected value for case %s: %v", fields[0], err)
				}
				if got, ok := want[fields[3]]; !ok || !reflect.DeepEqual(got, expected) {
					t.Fatalf("native case %s observation %s=%v, want %v", fields[0], fields[3], got, expected)
				}
			}

			projectDirectory := filepath.Join(directory, "native", "api"+strings.TrimSuffix(api, ".0"))
			root := t.TempDir()
			err := filepath.WalkDir(projectDirectory, func(path string, entry fs.DirEntry, err error) error {
				if err == nil && entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(projectDirectory, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("native source mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"OwnedGeneratedQueryProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation assertion: %+v; problem: %+v", result, result.Problem)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|generatedSOQLRelationship|")
			var got map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid Glade observation: %s", result.Problem.Message)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", columns[2], payload)
			}
		})
	}
}
