package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const visualforceFixturePackage = "github.com/glade-sh/glade/internal/visualforce"

func visualforceFixtureNames(count int) []string {
	names := make([]string, count)
	for index := range names {
		names[index] = fmt.Sprintf("TestVisualforce%03d", index)
	}
	return names
}

func visualforceFixtureEvents(names []string) string {
	var events strings.Builder
	for index, name := range names {
		elapsed := 0.001
		if index < 4 {
			elapsed = 100
		}
		fmt.Fprintf(&events, "{\"Action\":\"run\",\"Package\":%q,\"Test\":%q}\n", visualforceFixturePackage, name)
		fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", visualforceFixturePackage, name, elapsed)
	}
	fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", visualforceFixturePackage)
	return events.String()
}

func runVisualforceShardFixture(t *testing.T, index, discovery, planner, history, events string, nativeRC int) (string, error, string) {
	t.Helper()
	root := t.TempDir()
	artifacts := filepath.Join(root, "artifacts")
	for name, data := range map[string]string{"discovery": discovery, "events": events} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeApexFixtureExecutable(t, filepath.Join(root, "go"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == "test -list ^Test ./internal/visualforce" ]]; then cat "$FIXTURE_ROOT/discovery"; exit 0; fi
if [[ "$1" == "test" && "$2" == "-json" ]]; then
  printf '%s\n' "$*" >>"$FIXTURE_ROOT/calls"
  if [[ -s "$FIXTURE_ROOT/events" ]]; then
    cat "$FIXTURE_ROOT/events"
  else
    python3 - "$@" <<'PY'
import json, os, re, sys
arguments = sys.argv[1:]
selection = re.compile(arguments[arguments.index("-run") + 1])
package = "github.com/glade-sh/glade/internal/visualforce"
with open(os.path.join(os.environ["FIXTURE_ROOT"], "discovery")) as source:
    names = [line.strip() for line in source if line.startswith("Test")]
for name in names:
    if selection.fullmatch(name):
        print(json.dumps({"Action": "run", "Package": package, "Test": name}))
        print(json.dumps({"Action": "pass", "Package": package, "Test": name, "Elapsed": 0.001}))
print(json.dumps({"Action": "pass", "Package": package}))
PY
  fi
  exit "$FIXTURE_NATIVE_RC"
fi
exit 97
`)
	writeApexFixtureExecutable(t, filepath.Join(root, "renderer"), "#!/usr/bin/env bash\ncat >/dev/null\n")
	cmd := exec.Command("bash", "ci-go-test.sh", "visualforce-shard", index)
	cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "FIXTURE_ROOT="+root,
		"CI_SHARD_PLANNER="+planner, "CI_TESTLOG_RENDERER="+filepath.Join(root, "renderer"),
		"CI_VISUALFORCE_ARTIFACT_DIR="+artifacts, "CI_VISUALFORCE_HISTORY_PATH="+history,
		fmt.Sprintf("FIXTURE_NATIVE_RC=%d", nativeRC))
	out, err := cmd.CombinedOutput()
	return string(out), err, artifacts
}

func TestCIVisualforceShardsPreserveFullDiscoveryWithRealPlanner(t *testing.T) {
	root := t.TempDir()
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("build real planner: %v\n%s", err, out)
	}
	names := visualforceFixtureNames(264)
	discovery := strings.Join(names, "\n") + "\nok  \t" + visualforceFixturePackage + "\n"
	serialEvents, history := filepath.Join(root, "serial.json"), filepath.Join(root, "history.json")
	if err := os.WriteFile(serialEvents, []byte(visualforceFixtureEvents(names)), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "ci-go-test.sh", "visualforce-history-bootstrap", history, serialEvents).CombinedOutput(); err != nil {
		t.Fatalf("bootstrap serial history: %v\n%s", err, out)
	}
	invalidHistory := filepath.Join(root, "invalid-history.json")
	if err := os.WriteFile(invalidHistory, []byte("not validated JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name, history string
		weighted      bool
	}{
		{"validated serial history", history, true},
		{"invalid history fallback", invalidHistory, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dirs := make([]string, 4)
			var union []string
			var firstPlan string
			for index := range dirs {
				out, err, artifacts := runVisualforceShardFixture(t, strconv.Itoa(index), discovery, planner, fixture.history, "", 0)
				if err != nil {
					t.Fatalf("shard %d: %v\n%s", index, err, out)
				}
				dirs[index] = artifacts
				data, err := os.ReadFile(filepath.Join(artifacts, "discovery.txt"))
				if err != nil || string(data) != strings.Join(names, "\n")+"\n" {
					t.Fatalf("shard %d filtered full discovery: %s %v", index, data, err)
				}
				data, err = os.ReadFile(filepath.Join(artifacts, "plan.json"))
				var plan struct {
					HistoryUsed bool `json:"historyUsed"`
					Shards      []struct {
						Tests    []string `json:"tests"`
						Regex    string   `json:"regex"`
						Duration int64    `json:"estimatedDurationMillis"`
					} `json:"shards"`
				}
				if err != nil || json.Unmarshal(data, &plan) != nil || plan.HistoryUsed != fixture.weighted || len(plan.Shards) != 4 {
					t.Fatalf("shard %d plan=%s err=%v", index, data, err)
				}
				if index == 0 {
					firstPlan = string(data)
				} else if string(data) != firstPlan {
					t.Fatal("workers did not share the same plan")
				}
				selection := plan.Shards[index]
				if fixture.weighted {
					long := 0
					for _, name := range selection.Tests {
						if name <= "TestVisualforce003" {
							long++
						}
					}
					if long != 1 || selection.Duration >= 200000 {
						t.Fatalf("long tests were not separated: %#v", selection)
					}
				}
				data, err = os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
				want := "test -json -vet=off -count=1 -timeout=70m -run " + selection.Regex + " ./internal/visualforce\n"
				if err != nil || string(data) != want {
					t.Fatalf("native call=%s want=%s err=%v", data, want, err)
				}
				union = append(union, selection.Tests...)
			}
			sort.Strings(union)
			if !reflect.DeepEqual(union, names) {
				t.Fatalf("four-shard union changed discovery: %v", union)
			}
			output := filepath.Join(t.TempDir(), "refreshed.json")
			if out, err := runVisualforceHistory(t, output, dirs); err != nil {
				t.Fatalf("actual four-worker evidence rejected: %v\n%s", err, out)
			}
			assertVisualforceHistory(t, output, names)
		})
	}
}

func TestCIVisualforceShardRejectsIncompleteResultsAndDiscovery(t *testing.T) {
	_, dirs := visualforceHistoryFixture(t)
	planner := filepath.Join(t.TempDir(), "planner")
	writeApexFixtureExecutable(t, planner, "#!/usr/bin/env bash\ncat '"+filepath.Join(dirs[0], "plan.json")+"'\n")
	names := visualforceFixtureNames(8)
	discovery := strings.Join(names, "\n") + "\nok  \t" + visualforceFixturePackage + "\n"
	events := visualforceFixtureEvents(names[:2])
	for _, fixture := range []struct {
		name, discovery, events string
		nativeRC, wantRC        int
		wantNative              bool
	}{
		{"missing test", discovery, visualforceFixtureEvents(names[:1]), 0, 1, true},
		{"extra test", discovery, visualforceFixtureEvents(names[:3]), 0, 1, true},
		{"skipped test", discovery, strings.Replace(events, `"Action":"pass"`, `"Action":"skip"`, 1), 0, 1, true},
		{"nested skip", discovery, events + fmt.Sprintf("{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"TestVisualforce000/browser\"}\n", visualforceFixturePackage), 0, 1, true},
		{"nested fail", discovery, events + fmt.Sprintf("{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestVisualforce000/browser\"}\n", visualforceFixturePackage), 0, 1, true},
		{"native failure", discovery, strings.Replace(events, `"Action":"pass"`, `"Action":"fail"`, 1), 23, 23, true},
		{"missing package", discovery, strings.Replace(events, fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", visualforceFixturePackage), "", 1), 0, 1, true},
		{"duplicate discovery", "TestVisualforce000\n" + discovery, events, 0, 1, false},
		{"blank discovery line", "\n" + discovery, events, 0, 1, false},
		{"wrong package trailer", strings.Replace(discovery, visualforceFixturePackage, semaFixturePackage, 1), events, 0, 1, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			out, err, artifacts := runVisualforceShardFixture(t, "0", fixture.discovery, planner, "", fixture.events, fixture.nativeRC)
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != fixture.wantRC {
				t.Fatalf("exit=%v want=%d\n%s", err, fixture.wantRC, out)
			}
			_, callErr := os.Stat(filepath.Join(filepath.Dir(artifacts), "calls"))
			if (callErr == nil) != fixture.wantNative {
				t.Fatalf("native execution=%v want=%v", callErr == nil, fixture.wantNative)
			}
			if fixture.wantNative {
				data, err := os.ReadFile(filepath.Join(artifacts, "events.json"))
				if err != nil || string(data) != fixture.events {
					t.Fatalf("raw failure evidence not preserved: %s %v", data, err)
				}
			}
		})
	}
	for _, index := range []string{"", "4", "-1", "01"} {
		out, err, artifacts := runVisualforceShardFixture(t, index, discovery, planner, "", events, 0)
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 {
			t.Fatalf("invalid index %q exit=%v\n%s", index, err, out)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(artifacts), "calls")); !os.IsNotExist(err) {
			t.Fatalf("invalid index %q executed tests", index)
		}
	}
}

func TestCIVisualforceShardRequiresUniqueRemainingLaneMembership(t *testing.T) {
	for _, packages := range []string{"./internal/other\n", "./internal/visualforce\n./internal/visualforce\n"} {
		cmd := exec.Command("bash", "-c", `source ci-go-test.sh
package_lane_packages() { printf '%s' "$FIXTURE_PACKAGES"; }
run_visualforce_matrix_shard 0`)
		cmd.Env = append(os.Environ(), "CI_SHARD_PLANNER=", "CI_VISUALFORCE_ARTIFACT_DIR="+t.TempDir(), "FIXTURE_PACKAGES="+packages)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Visualforce must occur exactly once in the remaining-go package lane") {
			t.Fatalf("invalid membership accepted: %v\n%s", err, out)
		}
	}
}
