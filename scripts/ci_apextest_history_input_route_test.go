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

func TestCICase319HistoryInputRoute(t *testing.T) {
	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "scripts/ci-apex-history-input.sh") {
		t.Error("workflow does not call history input entry point")
	}
	names := make([]string, 12)
	shards := make([]map[string]any, 12)
	for i := range names {
		names[i] = fmt.Sprintf("TestBootstrap%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]}, "estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"predecessor", "missing-shard", "retry", "invalid-retry", "benchmark"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			evidence := filepath.Join(root, "evidence")
			dirs := writePlannedHistoryEvidence(t, evidence, fixturePackage, names, plan, func(i int, _ string) float64 { return float64(i + 1) })
			for i, dir := range dirs {
				if err := os.Rename(dir, filepath.Join(evidence, fmt.Sprintf("apex-shard-%d", i))); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "missing-shard" {
				if err := os.Remove(filepath.Join(evidence, "apex-shard-11", "events.json")); err != nil {
					t.Fatal(err)
				}
			}
			retry := `{"version":1,"package":"` + fixturePackage + `","complete":true,"tests":[{"name":"TestBootstrap00","durationMillis":42000}]}`
			if mode == "invalid-retry" {
				retry = `{"version":1,"package":"wrong","complete":true,"tests":[]}`
			}
			if err := os.WriteFile(filepath.Join(root, "profile.json"), []byte(retry), 0600); err != nil {
				t.Fatal(err)
			}
			stale := `{"version":1,"package":"` + fixturePackage + `","complete":true,"tests":[{"name":"TestBootstrap00","durationMillis":1}]}`
			history := filepath.Join(root, "history", "apextest-duration-history.json")
			if mode == "retry" || mode == "invalid-retry" || mode == "benchmark" {
				if err := os.MkdirAll(filepath.Dir(history), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(history, []byte(stale), 0600); err != nil {
					t.Fatal(err)
				}
			}
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			fake := `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FAKE_CALLS"
if [ "$1" = api ]; then echo 901; exit 0; fi
if [ "$1" = run ] && [ "$2" = download ]; then
  run=$3; shift 3
  name=""; pattern=""; dest=""
  while [ "$#" -gt 0 ]; do case "$1" in -n|--name) name=$2; shift 2;; -p|--pattern) pattern=$2; shift 2;; -D|--dir) dest=$2; shift 2;; *) exit 2;; esac; done
  mkdir -p "$dest"
  if [ -n "$name" ]; then [ "$run" = 777 ] && [ "$name" = apextest-scheduling-profile-777-1 ] || exit 1; cp "$FAKE_PROFILE" "$dest/profile.json"; else cp -R "$FAKE_EVIDENCE"/apex-shard-* "$dest/"; fi
  exit 0
fi
exit 2
`
			if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(root, "stage")
			cmd := exec.Command("bash", "ci-apex-history-input.sh")
			attempt := "1"
			if mode == "retry" || mode == "invalid-retry" || mode == "benchmark" {
				attempt = "2"
			}
			pair := "0"
			if mode == "benchmark" {
				pair = "7"
			}
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FAKE_EVIDENCE="+evidence, "FAKE_PROFILE="+filepath.Join(root, "profile.json"), "FAKE_CALLS="+filepath.Join(root, "calls"), "GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT="+attempt, "GITHUB_EVENT_NAME=push", "CI_DURATION_HISTORY_CACHE_REF=abc", "CI_APEXTEST_HISTORY_PATH="+history, "CI_APEXTEST_HISTORY_STAGE="+stage, "CI_APEXTEST_BENCHMARK_PAIR="+pair, "RUNNER_TEMP="+root)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("input route: %v\n%s", err, out)
			}
			data, err := os.ReadFile(filepath.Join(stage, "apextest-duration-history.json"))
			if mode == "missing-shard" {
				if !os.IsNotExist(err) {
					t.Fatalf("invalid shard published history: %s %v", data, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("missing staged input: %v\n%s", err, out)
			}
			switch mode {
			case "retry":
				if !strings.Contains(string(data), `"durationMillis":42000`) {
					t.Fatalf("retry did not override stale cache: %s", data)
				}
			case "invalid-retry", "benchmark":
				if string(data) != stale {
					t.Fatalf("lost valid cache fallback: %s", data)
				}
			case "predecessor":
				if !strings.Contains(string(data), `"durationMillis": 12000`) {
					t.Fatalf("predecessor observations missing: %s", data)
				}
			}
			calls, _ := os.ReadFile(filepath.Join(root, "calls"))
			if mode == "benchmark" && len(calls) != 0 {
				t.Fatalf("benchmark escaped isolation: %s", calls)
			}
		})
	}
}
