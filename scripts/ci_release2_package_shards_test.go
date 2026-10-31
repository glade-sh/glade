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

func TestCIRelease2FourShardPackageRoutesAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name        string
		lane        string
		packageName string
		packageArg  string
		artifactEnv string
	}{
		{"vm", "vm", "github.com/glade-sh/glade/internal/vm", "./internal/vm", "CI_VM_ARTIFACT_DIR"},
		{"gladecli", "gladecli", "github.com/glade-sh/glade/internal/gladecli", "./internal/gladecli", "CI_GLADECLI_ARTIFACT_DIR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			names := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
			discovered := append([]string(nil), names...)
			if tc.name == "gladecli" {
				discovered = append(discovered,
					"TestRunDoctorJSON",
					"TestRunDoctorReportsParser",
					"TestRunDoctorReportsProjectLocalDataEnvironment",
					"TestRunDoctorShortFlags",
				)
			}
			shards := make([]map[string]any, 4)
			for index, name := range names {
				shards[index] = map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 1000, "regex": "^(?:" + name + ")$"}
			}
			plan, err := json.Marshal(map[string]any{"version": 1, "package": tc.packageName, "historyUsed": true, "shards": shards})
			if err != nil {
				t.Fatal(err)
			}
			dirs := make([]string, 4)
			for index, name := range names {
				dirs[index] = runRelease2PackageShardFixture(t, tc, fmt.Sprint(index), strings.Join(discovered, "\n")+"\nok  \t"+tc.packageName+"\n", string(plan), release2PackagePassEvent(tc.packageName, name))
				filtered, err := os.ReadFile(filepath.Join(dirs[index], "discovery.txt"))
				if err != nil || string(filtered) != strings.Join(names, "\n")+"\n" {
					t.Fatalf("%s discovery was not restricted to its package authority: %v %s", tc.lane, err, filtered)
				}
			}
			output := filepath.Join(t.TempDir(), "history.json")
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh", tc.lane + "-history-refresh", output}, dirs...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s history failed: %v\n%s", tc.lane, err, out)
			}
			assertRelease2HistoryNames(t, output, names)
		})
	}
}

func TestCIRelease2VMHistoryRetainsValidatedDeferredOutcomes(t *testing.T) {
	t.Setenv("GLADE_DATAWEAVE_APEX_TEST_HOME", "")
	t.Setenv("CI_VM_HISTORY_COMPLETE", "1")
	tc := struct {
		name        string
		lane        string
		packageName string
		packageArg  string
		artifactEnv string
	}{"vm", "vm", "github.com/glade-sh/glade/internal/vm", "./internal/vm", "CI_VM_ARTIFACT_DIR"}
	names := []string{"TestAlpha", "TestBeta", "TestDelta", "TestExecDataWeaveScriptResultCarriers"}
	shards := make([]map[string]any, 4)
	for index, name := range names {
		shards[index] = map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 1000, "regex": "^(?:" + name + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": tc.packageName, "historyUsed": true, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 4)
	for index, name := range names {
		events := release2PackagePassEvent(tc.packageName, name)
		if name == "TestExecDataWeaveScriptResultCarriers" {
			events = release2PackageDeferredEvent(tc.packageName, name)
		}
		dirs[index] = runRelease2PackageShardFixture(t, tc, fmt.Sprint(index), strings.Join(names, "\n")+"\nok  \t"+tc.packageName+"\n", string(plan), events)
	}
	output := filepath.Join(t.TempDir(), "history.json")
	cmd := exec.Command("bash", append([]string{"ci-go-test.sh", "vm-history-refresh", output}, dirs...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("VM deferred history failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Complete bool `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if !history.Complete || len(history.Tests) != len(names) {
		t.Fatalf("incomplete VM history: %s", data)
	}
	for index, name := range names {
		wantDuration := int64(1000)
		if name == "TestExecDataWeaveScriptResultCarriers" {
			wantDuration = 0
		}
		if history.Tests[index].Name != name || history.Tests[index].DurationMillis != wantDuration {
			t.Fatalf("unexpected VM history row %d: %s", index, data)
		}
	}
}

func runRelease2PackageShardFixture(t *testing.T, tc struct {
	name        string
	lane        string
	packageName string
	packageArg  string
	artifactEnv string
}, index, discovery, plan, events string) string {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"discovery": discovery,
		"events":    events,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goScript := `#!/usr/bin/env bash
set -u
if [[ "$1" == "test" && "$*" == *" -list ^Test "* ]]; then cat "$FIXTURE_DISCOVERY"; exit 0; fi
if [[ "$1" == "test" && "$2" == "-json" ]]; then printf '%s\n' "$*" >>"$FIXTURE_CALLS"; cat "$FIXTURE_EVENTS"; exit 0; fi
echo "unexpected go invocation: $*" >&2
exit 97
`
	plannerScript := "#!/usr/bin/env bash\ncat <<'EOF'\n" + plan + "\nEOF\n"
	rendererScript := "#!/usr/bin/env bash\ncat >/dev/null\n"
	for name, body := range map[string]string{"go": goScript, "planner": plannerScript, "renderer": rendererScript} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	artifacts := filepath.Join(root, "artifacts")
	calls := filepath.Join(root, "calls")
	cmd := exec.Command("bash", "ci-go-test.sh", tc.lane+"-shard", index)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CI_SHARD_PLANNER="+filepath.Join(binDir, "planner"),
		"CI_TESTLOG_RENDERER="+filepath.Join(binDir, "renderer"),
		tc.artifactEnv+"="+artifacts,
		"FIXTURE_DISCOVERY="+filepath.Join(root, "discovery"),
		"FIXTURE_EVENTS="+filepath.Join(root, "events"),
		"FIXTURE_CALLS="+calls,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s shard %s failed: %v\n%s", tc.lane, index, err, out)
	}
	data, err := os.ReadFile(calls)
	if err != nil || !strings.Contains(string(data), "-count=1 -timeout=30m") {
		t.Fatalf("%s shard did not use its uncached CI route: %v %s", tc.lane, err, data)
	}
	return artifacts
}

func release2PackagePassEvent(packageName, name string) string {
	return `{"Action":"pass","Package":"` + packageName + `","Test":"` + name + `","Elapsed":1}` + "\n" +
		`{"Action":"pass","Package":"` + packageName + `","Elapsed":1}` + "\n"
}

func release2PackageDeferredEvent(packageName, name string) string {
	return `{"Action":"output","Package":"` + packageName + `","Test":"` + name + `","Output":"    deferred_test.go:1: requires explicitly installed DataWeave toolchain\n"}` + "\n" +
		`{"Action":"skip","Package":"` + packageName + `","Test":"` + name + `","Elapsed":0}` + "\n" +
		`{"Action":"pass","Package":"` + packageName + `","Elapsed":1}` + "\n"
}

func TestCIRelease2PackageHistoryRejectsTamperedNestedSkips(t *testing.T) {
	for _, tc := range []struct {
		name, lane, packageName, packageArg, artifactEnv string
	}{
		{"vm", "vm", "github.com/glade-sh/glade/internal/vm", "./internal/vm", "CI_VM_ARTIFACT_DIR"},
		{"gladecli", "gladecli", "github.com/glade-sh/glade/internal/gladecli", "./internal/gladecli", "CI_GLADECLI_ARTIFACT_DIR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			names := []string{"TestAlpha", "TestBeta", "TestDelta", "TestGamma"}
			discovered := append([]string(nil), names...)
			if tc.name == "gladecli" {
				discovered = append(discovered, "TestRunDoctorJSON", "TestRunDoctorReportsParser", "TestRunDoctorReportsProjectLocalDataEnvironment", "TestRunDoctorShortFlags")
			}
			shards := make([]map[string]any, 4)
			for index, name := range names {
				shards[index] = map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 1000, "regex": "^(?:" + name + ")$"}
			}
			plan, err := json.Marshal(map[string]any{"version": 1, "package": tc.packageName, "historyUsed": true, "shards": shards})
			if err != nil {
				t.Fatal(err)
			}
			dirs := make([]string, 4)
			for index, name := range names {
				dirs[index] = runRelease2PackageShardFixture(t, tc, fmt.Sprint(index), strings.Join(discovered, "\n")+"\nok  \t"+tc.packageName+"\n", string(plan), release2PackagePassEvent(tc.packageName, name))
			}
			file, err := os.OpenFile(filepath.Join(dirs[0], "events.json"), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(file, "{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"TestAlpha/unapproved\"}\n", tc.packageName)
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh", tc.lane + "-history-refresh", filepath.Join(t.TempDir(), "history.json")}, dirs...)...)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("collector accepted an unapproved nested skip:\n%s", out)
			}
		})
	}
}
