package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThrowSuccessorScopeProjectAndCachedAnonymousExecNativeRoutes(t *testing.T) {
	const dir = "../sema/testdata/throw_successor_scope"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, c := range cases {
		bodies[c.ID] = c.Body
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(cases) != 6 || len(rows) != 13 {
		t.Fatalf("incomplete native scope matrix: %v", err)
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			if bodies[row[2]] == "" || row[1] != "anonymous" {
				t.Fatal("missing anonymous source")
			}
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			for _, route := range []string{"project", "cached"} {
				t.Run(route, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", bodies[row[2]]}, &stdout, &stderr)
					output := stdout.String() + stderr.String()
					if code == 0 {
						t.Fatalf("native rejects or throws; exec succeeded: %s", output)
					}
					if row[3] == "rejected" {
						// Exec renders the local diagnostic; the semantic route
						// independently checks the captured native wording.
						if !strings.Contains(output, "GLADESEMA030") || !strings.Contains(output, "instruction is unreachable") {
							t.Fatalf("native compile error %q; exec=%s", row[4], output)
						}
					} else if strings.Contains(output, "GLADESEMA") || !strings.Contains(output, strings.TrimPrefix(row[5], "System.")) {
						t.Fatalf("native compiles and throws %q; exec=%s", row[5], output)
					}
				})
			}
		})
	}
}
