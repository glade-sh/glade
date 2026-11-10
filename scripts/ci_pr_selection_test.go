package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCIPRPackageSelection(t *testing.T) {
	root := t.TempDir()
	graph := `[{"ImportPath":"example/a","Dir":"/repo/a"},{"ImportPath":"example/b","Dir":"/repo/b","Imports":["example/a"]},{"ImportPath":"example/c","Dir":"/repo/c","TestImports":["example/a"]},{"ImportPath":"example/d","Dir":"/repo/d","Imports":["example/b"]}]`
	manifest := `{"version":1,"lanes":{"remaining-go":["example/a","example/b","example/c","example/d"]}}`
	for name, data := range map[string]string{"graph.json": graph, "manifest.json": manifest} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, paths string
		want        []string
	}{
		{"direct and test importers", "a/a.go\n", []string{"example/a", "example/b", "example/c"}},
		{"non go fixture", "a/testdata/input.cls\n", []string{"example/a", "example/b", "example/c"}},
		{"unrelated docs", "docs/readme.md\n", []string{}},
		{"multiple", "a/a.go\nd/d.go\n", []string{"example/a", "example/b", "example/c", "example/d"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("python3", "ci-pr-select.py", "--root", "/repo", "--graph", filepath.Join(root, "graph.json"), "--manifest", filepath.Join(root, "manifest.json"))
			cmd.Stdin = strings.NewReader(tc.paths)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("selector: %v\n%s", err, out)
			}
			var got []string
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("invalid output: %v: %s", err, out)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
