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

func TestFlowTypedOwnerDecisionNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/flow_typed_owner_decision"
	capture, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatal("incomplete native capture")
	}
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 3 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("incomplete native observations")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		columns := strings.SplitN(row, "\t", 3)
		if len(columns) != 3 {
			t.Fatal("invalid native observation")
		}
		api := columns[0]
		if (api != "62.0" && api != "67.0") || columns[1] != "istest" || seen[api] {
			t.Fatal("unknown or duplicate native observation")
		}
		seen[api] = true
		t.Run(api+"/istest", func(t *testing.T) {
			root := t.TempDir()
			projectDirectory := filepath.Join(directory, "project")
			err := filepath.WalkDir(projectDirectory, func(path string, entry fs.DirEntry, err error) error {
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
				source := strings.ReplaceAll(string(body), "62.0", api)
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256([]byte(source)))) {
					t.Fatalf("native source mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"GroveTypedOwnerObservationTest"}, SelectedMethod: "observeOwner", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing native assertion transport: %+v", result)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|typedOwner|")
			var got, want map[string]string
			if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[2]), &want) != nil {
				t.Fatalf("invalid observation: %s", result.Problem.Message)
			}
			if len(want) != 2 {
				t.Fatal("native observation must cover both owner types")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", columns[2], payload)
			}
		})
	}
}
