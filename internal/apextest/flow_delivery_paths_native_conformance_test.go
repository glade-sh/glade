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

func TestFlowDeliveryPathsNativeConformanceIsTestProjectAPIRoutes(t *testing.T) {
	const directory = "testdata/conformance/flow_delivery_paths"
	capture, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatal("incomplete native capture")
	}
	table, err := os.ReadFile(filepath.Join(directory, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{}
	for _, row := range strings.Split(strings.TrimSpace(string(table)), "\n")[1:] {
		cells := strings.Split(row, "\t")
		if len(cells) != 3 || cells[1] != "istest" || cases[cells[0]] != "" {
			t.Fatalf("invalid case: %s", row)
		}
		cases[cells[0]] = cells[2]
	}
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 1+2*len(cases) || rows[0] != "source_api\troute\tcase\tobservation" {
		t.Fatal("expected every case at both native APIs")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		cells := strings.SplitN(row, "\t", 4)
		if len(cells) != 4 || (cells[0] != "62.0" && cells[0] != "67.0") || cells[1] != "istest" || cases[cells[2]] == "" {
			t.Fatalf("invalid observation: %s", row)
		}
		api, method := cells[0], cells[2]
		key := api + "/" + method
		if seen[key] {
			t.Fatalf("duplicate observation %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var want map[string][]string
			if err := json.Unmarshal([]byte(cells[3]), &want); err != nil || len(want) == 0 {
				t.Fatalf("invalid observation: %s", cells[3])
			}
			root := t.TempDir()
			projectDirectory := filepath.Join(directory, "api"+strings.TrimSuffix(api, ".0"))
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
				// Each local slice retains the subscriber for its captured start shape.
				if strings.HasSuffix(name, ".flow-meta.xml") {
					selected := ""
					switch method {
					case "asyncPath":
						selected = "Cedar_async"
					case "scheduledPath":
						selected = "Cedar_scheduled"
					case "eventExplicit", "eventStop":
						selected = "Cedar_event"
					case "cdcFlowOnly":
						selected = "Cedar_cdc"
					}
					if selected == "" || !strings.HasPrefix(entry.Name(), selected+".") {
						return nil
					}
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
			run := Run(index, Options{SelectedClasses: []string{"CedarDeliveryObservationTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil {
				t.Fatalf("missing observation assertion: %+v", result)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|"+method+"|")
			var got map[string][]string
			if !strings.HasSuffix(result.Problem.Type, "AssertException") || !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("missing Glade observation: %+v", result.Problem)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", cells[3], payload)
			}
		})
	}
}
