package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// permsetMutingIDPattern matches the 15 and 18 character record ids that native
// rows carry. Ids differ per org, so both sides compare as "<id>".
var permsetMutingIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{15}([A-Za-z0-9]{3})?$`)

// normalizePermsetMutingRow drops what a local run cannot share with a native
// org (record ids, REST attributes, stack frames of the capture's own classes)
// and keeps every permission, error and strip observation.
func normalizePermsetMutingRow(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if key == "attributes" || key == "stackTrace" {
				continue
			}
			item = normalizePermsetMutingRow(item)
			if list, ok := item.([]any); ok && key == "records" {
				// SOQL without ORDER BY: compare record sets, not native order.
				sort.SliceStable(list, func(i, j int) bool {
					a, _ := json.Marshal(list[i])
					b, _ := json.Marshal(list[j])
					return string(a) < string(b)
				})
			}
			out[key] = item
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizePermsetMutingRow(item)
		}
		return out
	case string:
		if permsetMutingIDPattern.MatchString(typed) && strings.ContainsAny(typed, "0123456789") {
			return "<id>"
		}
	}
	return value
}

func copyPermsetMutingProject(t *testing.T, project, api string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPermsetMutingNativeConformance replays the native muting capture (test
// route, restricted user holding only the permission set group) at API 62 and
// 67. The anonymous route is the administrator control and has no group
// assignment, so it is not a muting observation. API 68 rows equal API 67.
func TestPermsetMutingNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "permset_muting")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 37 || strings.Join(rows[0], ",") != "probe-id,api,route,outcome-or-value,exact-message-text,raw-json" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("synthetic project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"PermMuteObserve"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 {
				t.Fatalf("expected one suite: %+v", run.Summary())
			}
			observed := map[string]any{}
			for _, result := range run.Suites[0].Cases {
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("%s: missing assertion transport: %+v", result.MethodName, result.Problem)
				}
				_, payload, _ := strings.Cut(result.Problem.Message, "OBS|")
				var row map[string]any
				if err := json.Unmarshal([]byte(payload), &row); err != nil {
					t.Fatalf("%s: invalid observation %q", result.MethodName, payload)
				}
				probe, _ := row["probeId"].(string)
				if probe == "" {
					probe = "actor"
				}
				observed[row["caseId"].(string)+"."+probe] = row
			}
			compared := 0
			for _, row := range rows[1:] {
				if row[1] != api || row[2] != "test" {
					continue
				}
				parts := strings.Split(row[0], ".")
				key := parts[0] + "." + strings.Join(parts[1:len(parts)-1], ".")
				t.Run(key, func(t *testing.T) {
					var want any
					if err := json.Unmarshal([]byte(row[5]), &want); err != nil {
						t.Fatal(err)
					}
					got, ok := observed[key]
					if !ok {
						t.Fatalf("no Glade observation for %s", key)
					}
					if !reflect.DeepEqual(normalizePermsetMutingRow(got), normalizePermsetMutingRow(want)) {
						gotJSON, _ := json.Marshal(normalizePermsetMutingRow(got))
						wantJSON, _ := json.Marshal(normalizePermsetMutingRow(want))
						t.Fatalf("native=%s\nGlade=%s", wantJSON, gotJSON)
					}
				})
				compared++
			}
			if compared != 18 {
				t.Fatalf("compared %d native rows at %s, want 18", compared, api)
			}
		})
	}
}

// TestPermsetMutingDeployRejections checks that Glade reports the native
// component errors for muting permission sets whose object dependencies are
// not muted, and the group component error that follows them.
func TestPermsetMutingDeployRejections(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "permset_muting")
	f, err := os.Open(filepath.Join(dir, "deploy.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("native rejection rows=%d, want 3", len(rows)-1)
	}
	for _, row := range rows[1:] {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(row[0]+"/"+api, func(t *testing.T) {
				var want []string
				if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
					t.Fatal(err)
				}
				root := copyPermsetMutingProject(t, filepath.Join(dir, "reject", row[0]), api)
				var got []string
				for _, d := range loadTestIndex(t, root).Diagnostics {
					if d.Code == "GLADEPERMMUTE001" {
						got = append(got, d.Message)
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%q; Glade=%q", want, got)
				}
			})
		}
	}
	t.Run("accepted muting", func(t *testing.T) {
		root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), "67.0")
		for _, d := range loadTestIndex(t, root).Diagnostics {
			if d.Code == "GLADEPERMMUTE001" {
				t.Fatalf("native accepted this metadata; Glade: %s", d.Message)
			}
		}
	})
}
