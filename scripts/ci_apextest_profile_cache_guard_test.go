package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCICase319CachedProfileRejectsInvalidAndKeepsBenchmarkIsolated(t *testing.T) {
	pkg := "github.com/glade-sh/glade/internal/apextest"
	stale := fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestA","durationMillis":1}]}`, pkg)
	for _, tc := range []struct{ name, profile, pair string }{
		{"wrong-package", `{"version":1,"package":"wrong","complete":true,"tests":[{"name":"TestA","durationMillis":42}]}`, "0"},
		{"negative", fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestA","durationMillis":-1}]}`, pkg), "0"},
		{"benchmark", fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestA","durationMillis":42}]}`, pkg), "7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			fake := `#!/bin/bash
printf called >> "$FAKE_CALLS"
exit 1
`
			if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			history := filepath.Join(root, "history.json")
			profile := filepath.Join(root, "profile.json")
			if err := os.WriteFile(history, []byte(stale), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(profile, []byte(tc.profile), 0600); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(root, "stage")
			cmd := exec.Command("bash", "ci-apex-history-input.sh")
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FAKE_CALLS="+filepath.Join(root, "calls"), "GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT=2", "GITHUB_EVENT_NAME=push", "CI_DURATION_HISTORY_CACHE_REF=abc", "CI_APEXTEST_HISTORY_PATH="+history, "CI_APEXTEST_RETRY_PROFILE_PATH="+profile, "CI_APEXTEST_HISTORY_STAGE="+stage, "CI_APEXTEST_BENCHMARK_PAIR="+tc.pair, "RUNNER_TEMP="+root)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("stage: %v %s", err, out)
			}
			data, err := os.ReadFile(filepath.Join(stage, "apextest-duration-history.json"))
			if err != nil || strings.TrimSpace(string(data)) != stale {
				t.Fatalf("cached input was accepted or predecessor lost: %s %v", data, err)
			}
			if tc.name == "benchmark" {
				if _, err := os.Stat(filepath.Join(root, "calls")); !os.IsNotExist(err) {
					t.Fatalf("benchmark performed artifact lookup: %v", err)
				}
			}
		})
	}
}
