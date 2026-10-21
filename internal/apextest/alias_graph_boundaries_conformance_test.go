package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAliasGraphBoundariesNativeConformance(t *testing.T) {
	runStoredGraphNativeConformance(t, "alias_graph_boundaries", "AliasGraphProbeTest", 16)
}

func TestPrefixRecordIdentityNativeConformance(t *testing.T) {
	runStoredGraphNativeConformance(t, "prefix_record_identity", "PrefixIdentityProbeTest", 1)
}

func runStoredGraphNativeConformance(t *testing.T, family, class string, rowCount int) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", family)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var capture struct {
				SourceAPI    string
				SourceSHA256 map[string]string
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &capture); err != nil {
				t.Fatal(err)
			}
			if capture.SourceAPI != api {
				t.Fatal("native source API changed")
			}
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.HasPrefix(name, ".sf/") {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				body = []byte(strings.ReplaceAll(string(body), "67.0", api))
				sum := sha256.Sum256(body)
				if capture.SourceSHA256[filepath.ToSlash(name)] != hex.EncodeToString(sum[:]) {
					t.Fatalf("native source differs: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			raw, err = os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(lines) != rowCount+1 || lines[0] != "source_api\troute\tvariant\tobservation" {
				t.Fatal("incomplete native graph table")
			}
			index := loadTestIndex(t, root)
			results := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
			if len(results.Suites) != 1 || len(results.Suites[0].Cases) != rowCount {
				t.Fatalf("native methods not run: %+v", results.Summary())
			}
			seen := map[string]bool{}
			for _, line := range lines[1:] {
				columns := strings.SplitN(line, "\t", 4)
				if len(columns) != 4 || columns[0] != api || columns[1] != "istest" || seen[columns[2]] {
					t.Fatalf("invalid native row: %s", line)
				}
				seen[columns[2]] = true
				t.Run(columns[2], func(t *testing.T) {
					for _, result := range results.Suites[0].Cases {
						if result.MethodName != columns[2] {
							continue
						}
						if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatal("missing native assertion transport")
						}
						_, payload, found := strings.Cut(result.Problem.Message, "OBS|")
						var got, want any
						if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil {
							t.Fatalf("native=%s; local=%s", columns[3], result.Problem.Message)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s; local=%s", columns[3], payload)
						}
						return
					}
					t.Fatal("native method not discovered")
				})
			}
		})
	}
}
