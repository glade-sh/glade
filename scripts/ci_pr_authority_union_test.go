package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPRAuthorityUnion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutation string
		ok       bool
	}{
		{"complete", "", true}, {"missing worker", "missing", false},
		{"divergent discovery", "discovery", false}, {"divergent plan", "plan", false},
		{"duplicate terminal", "duplicate", false}, {"skipped terminal", "skip", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := semaPairFixture(t)
			root := filepath.Join(f.root, "input")
			plan := map[string]any{"include": []any{
				map[string]any{"shard": 20, "packages": []string{semaFixturePackage}, "pool": []string{}, "browser": false},
				map[string]any{"shard": 21, "packages": []string{semaFixturePackage}, "pool": []string{}, "browser": false},
			}}
			for i, src := range f.shards {
				dst := filepath.Join(root, fmt.Sprintf("pr-authority-1-1-%d", 20+i), "shard")
				if tc.mutation == "missing" && i == 1 {
					continue
				}
				if err := os.MkdirAll(dst, 0700); err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(src)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					b, err := os.ReadFile(filepath.Join(src, entry.Name()))
					if err != nil {
						t.Fatal(err)
					}
					if i == 1 {
						if tc.mutation == "discovery" && entry.Name() == "discovery.txt" {
							b = append(b, []byte("TestZExtra\n")...)
						}
						if tc.mutation == "plan" && entry.Name() == "plan.json" {
							b = []byte(`{}`)
						}
						if tc.mutation == "duplicate" && entry.Name() == "events.json" {
							b = append(b, b...)
						}
						if tc.mutation == "skip" && entry.Name() == "events.json" {
							b = []byte(strings.Replace(string(b), `"Action":"pass"`, `"Action":"skip"`, 1))
						}
					}
					if err := os.WriteFile(filepath.Join(dst, entry.Name()), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
				writeJSONFixture(t, filepath.Join(filepath.Dir(dst), "identity.json"), map[string]any{"shard": 20 + i})
			}
			planPath := filepath.Join(f.root, "plan.json")
			writeJSONFixture(t, planPath, plan)
			cmd := exec.Command("python3", "ci-pr-authority-union.py", planPath, root, filepath.Join(f.root, "out"))
			cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME=pull_request", "GITHUB_HEAD_REF=feature/test")
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("union error=%v output=%s", err, out)
			}
			if tc.ok {
				data, err := os.ReadFile(filepath.Join(f.root, "out", "summary.json"))
				if err != nil || !json.Valid(data) {
					t.Fatalf("summary %s %v", data, err)
				}
			}
		})
	}
}

func TestCIPRAuthorityWorkflow(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, marker := range []string{"pr-authority-${{ github.run_id }}-${{ github.run_attempt }}-${{ matrix.shard }}", "ci-pr-packages.sh", "CI_SEMA_ARTIFACT_DIR"} {
		if !strings.Contains(jobs["pr-tests"], marker) {
			t.Errorf("PR worker missing %q", marker)
		}
	}
	if !strings.Contains(jobs["pr-authority-union"], "ci-pr-authority-union.py") || !strings.Contains(jobs["required-ci"], "      - pr-authority-union\n") {
		t.Fatal("PR authority union must gate Required CI")
	}
}
