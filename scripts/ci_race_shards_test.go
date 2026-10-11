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

func TestCIRaceApextestDeferredPolicy(t *testing.T) {
	for _, fixture := range []struct {
		name, reason, configured, suffix, test string
		valid                                  bool
	}{
		{name: "exact deferred prerequisite", valid: true},
		{name: "wrong reason", reason: "another missing dependency"},
		{name: "configured toolchain", configured: "/explicit/toolchain"},
		{name: "unrecognized test", test: "TestNotAdmitted"},
		{name: "unrecognized subtest", suffix: "/child"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			testName := fixture.test
			if testName == "" {
				testName = "TestRunDataWeaveTypedAdmitted"
			}
			names := raceApextestTestNames()
			names[0] = testName
			goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
			planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			out, err := runRacePackage(t, root, []string{
				"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
				"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(root, "calls.log"),
				"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "TMPDIR=" + root,
				"CI_RACE_DEFERRED_TEST=" + testName, "CI_RACE_DEFERRED_REASON=" + fixture.reason,
				"CI_RACE_DEFERRED_SUFFIX=" + fixture.suffix, "GLADE_DATAWEAVE_APEX_TEST_HOME=" + fixture.configured,
			}, "./internal/apextest", "internal-apextest")
			if (err == nil) != fixture.valid {
				t.Fatalf("valid=%v, err=%v\n%s", fixture.valid, err, out)
			}
			if fixture.valid {
				data, err := os.ReadFile(filepath.Join(root, "ci-artifacts/race/internal-apextest/union-validation.json"))
				var result struct {
					Discovered int              `json:"discovered_count"`
					Deferred   []map[string]any `json:"deferred"`
					Valid      bool             `json:"valid"`
				}
				if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != 16 || len(result.Deferred) != 1 || result.Deferred[0]["test"] != testName {
					t.Fatalf("deferred summary=%s err=%v", data, err)
				}
			}
		})
	}
}

func TestCIRaceApextestAggregateRevalidatesDeferredPrerequisites(t *testing.T) {
	t.Setenv("GLADE_DATAWEAVE_APEX_TEST_HOME", "")
	root := t.TempDir()
	head := strings.Repeat("b", 40)
	names := raceApextestTestNames()
	names[0] = "TestRunDataWeaveTypedAdmitted"
	goCommand := writeRaceFixture(t, "fake-go", raceApextestGoFixture())
	planner := writeRaceFixture(t, "fake-planner", raceApextestPlannerFixture())
	resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
	git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
	roots := make([]string, 3)
	for index, assignment := range []string{"0,3,5,8,11,13", "4,7,12,15", "1,2,6,9,10,14"} {
		runner := string(rune('a' + index))
		workdir := filepath.Join(root, runner)
		if err := os.MkdirAll(workdir, 0o700); err != nil {
			t.Fatal(err)
		}
		out, err := runRacePackage(t, workdir, []string{
			"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
			"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + filepath.Join(workdir, "calls.log"),
			"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
			"CI_RACE_APEXTEST_RUNNER=" + runner, "CI_RACE_APEXTEST_SHARD_INDEXES=" + assignment,
			"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "TMPDIR=" + workdir,
			"CI_RACE_DEFERRED_TEST=" + names[0],
		}, "./internal/apextest", "internal-apextest")
		if err != nil {
			t.Fatalf("runner %s failed: %v\n%s", runner, err, out)
		}
		roots[index] = filepath.Join(workdir, "ci-artifacts/race/internal-apextest")
	}
	script, err := filepath.Abs("ci-race-apextest-aggregate.sh")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "union.json")
	args := append([]string{script}, roots...)
	args = append(args, output, head)
	if out, err := exec.Command("bash", args...).CombinedOutput(); err != nil {
		t.Fatalf("aggregate rejected deferred prerequisite: %v\n%s", err, out)
	}
	data, err := os.ReadFile(output)
	var result struct {
		Deferred []map[string]any `json:"deferred"`
		Valid    bool             `json:"valid"`
	}
	if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || len(result.Deferred) != 1 || result.Deferred[0]["test"] != names[0] {
		t.Fatalf("aggregate deferred summary=%s err=%v", data, err)
	}
	t.Setenv("GLADE_DATAWEAVE_APEX_TEST_HOME", "/explicit/toolchain")
	if out, err := exec.Command("bash", args...).CombinedOutput(); err == nil {
		t.Fatalf("aggregate accepted configured prerequisite skip: %s", out)
	}
}

func TestCIRaceBrowserRunnersAndAggregateCoverDiscovery(t *testing.T) {
	for _, topology := range []struct {
		pkg, slug string
		count     int
	}{
		{"./internal/visualforce", "internal-visualforce", 4},
		{"./internal/lwc/compile", "internal-lwc-compile", 8},
	} {
		t.Run(topology.slug, func(t *testing.T) {
			packageName := "github.com/glade-sh/glade/" + strings.TrimPrefix(topology.pkg, "./")
			names := make([]string, topology.count)
			wantCounts := make([]int, topology.count)
			for index := range names {
				names[index] = fmt.Sprintf("TestCase%02d", index)
				wantCounts[index] = 1
			}

			root := t.TempDir()
			head := strings.Repeat("a", 40)
			goCommand := writeRaceFixture(t, "fake-go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"-list ."* ]]; then
  printf '%b' "$CI_RACE_TEST_NAMES"
  printf 'ok   %s 0.001s\n' "$CI_RACE_TEST_PACKAGE"
  exit 0
fi
printf 'native %s\n' "$*" >>"$CI_RACE_CALL_LOG"
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  if [[ "$*" == *"$name"* ]]; then
    printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\n' "$CI_RACE_TEST_PACKAGE" "$name"
  fi
done <<<"$CI_RACE_TEST_NAMES"
printf '{"Action":"pass","Package":"%s"}\n' "$CI_RACE_TEST_PACKAGE"
`)
			planner := writeRaceFixture(t, "fake-planner", racePlannerFixture())
			resource := writeRaceFixture(t, "fake-resource", raceResourceFixture())
			git := writeRaceFixture(t, "fake-git", "#!/usr/bin/env bash\nprintf '%s\\n' "+head+"\n")
			roots := make([]string, topology.count)
			for index := range roots {
				workdir := filepath.Join(root, fmt.Sprintf("runner-%d", index))
				if err := os.MkdirAll(workdir, 0o700); err != nil {
					t.Fatal(err)
				}
				log := filepath.Join(workdir, "calls.log")
				out, err := runRacePackage(t, workdir, []string{
					"CI_RACE_GO_COMMAND=" + goCommand, "CI_RACE_SHARD_PLANNER=" + planner,
					"CI_RACE_RESOURCE_RUNNER=" + resource, "CI_RACE_CALL_LOG=" + log,
					"CI_RACE_GIT_COMMAND=" + git, "CI_RACE_HEAD_SHA=" + head,
					"CI_RACE_TEST_NAMES=" + strings.Join(names, "\n") + "\n", "CI_RACE_TEST_PACKAGE=" + packageName,
					fmt.Sprintf("CI_RACE_SHARD_INDEX=%d", index),
				}, topology.pkg, topology.slug)
				if err != nil {
					t.Fatalf("runner %d: %v\n%s", index, err, out)
				}
				calls, err := os.ReadFile(log)
				if err != nil || strings.Count(string(calls), "native test -json -race -count=1 -timeout=60m -run") != 1 || !strings.Contains(string(calls), fmt.Sprintf("start race-%s-shard-%d", topology.slug, index)) {
					t.Fatalf("runner %d must execute its one assigned race shard: %s, %v", index, calls, err)
				}
				roots[index] = filepath.Join(workdir, "ci-artifacts/race", topology.slug)
			}
			aggregate, err := filepath.Abs("ci-race-shard-aggregate.sh")
			if err != nil {
				t.Fatal(err)
			}
			for _, fixture := range []struct {
				name   string
				mutate func(t *testing.T, roots []string)
				valid  bool
			}{
				{name: "exact union", valid: true},
				{name: "missing shard", mutate: func(t *testing.T, roots []string) { roots[len(roots)-1] = filepath.Join(root, "missing") }},
				{name: "duplicate runner", mutate: func(t *testing.T, roots []string) { roots[len(roots)-1] = roots[0] }},
				{name: "omitted terminal", mutate: func(t *testing.T, roots []string) {
					path := filepath.Join(roots[0], "shard-0/events.json")
					if err := os.WriteFile(path, []byte(fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q}\n", packageName)), 0o600); err != nil {
						t.Fatal(err)
					}
				}},
				{name: "unreported subtest skip", mutate: func(t *testing.T, roots []string) {
					path := filepath.Join(roots[0], "shard-0/events.json")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					data = append([]byte(fmt.Sprintf("{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"TestCase00/subtest\"}\n", packageName)), data...)
					if err := os.WriteFile(path, data, 0o600); err != nil {
						t.Fatal(err)
					}
				}},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					copies := make([]string, topology.count)
					for index, source := range roots {
						copies[index] = filepath.Join(t.TempDir(), "artifact")
						if err := os.CopyFS(copies[index], os.DirFS(source)); err != nil {
							t.Fatal(err)
						}
					}
					if fixture.mutate != nil {
						fixture.mutate(t, copies)
					}
					output := filepath.Join(t.TempDir(), "union.json")
					args := append([]string{aggregate, topology.pkg, output, head}, copies...)
					out, err := exec.Command("bash", args...).CombinedOutput()
					if (err == nil) != fixture.valid {
						t.Fatalf("valid=%v, err=%v\n%s", fixture.valid, err, out)
					}
					if fixture.valid {
						data, err := os.ReadFile(output)
						var result struct {
							Valid      bool  `json:"valid"`
							Discovered int   `json:"discovered_count"`
							Counts     []int `json:"shard_counts"`
						}
						if err != nil || json.Unmarshal(data, &result) != nil || !result.Valid || result.Discovered != topology.count || fmt.Sprint(result.Counts) != fmt.Sprint(wantCounts) {
							t.Fatalf("invalid aggregate %s err=%v", data, err)
						}
					}
				})
			}
		})
	}
}

func TestCIRaceWorkflowSchedulesBrowserPackagesSeparately(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start, end := strings.Index(workflow, "  race-browser:"), strings.Index(workflow, "  race-browser-aggregate:")
	if start < 0 || end <= start || start > strings.Index(workflow, "  race:\n") {
		t.Fatal("browser race packages must use a dedicated early matrix")
	}
	job := workflow[start:end]
	for _, marker := range []string{"needs: plan", "max-parallel: 8", "CI_RACE_SHARD_INDEX: ${{ matrix.shard }}", "CI_RACE_HEAD_SHA: ${{ github.sha }}", `scripts/ci-race-test.sh "$PACKAGE" "$SLUG"`, "GLADE_LWC_PLAYWRIGHT_MODULE: ${{ github.workspace }}/lwcruntime/node_modules/playwright", `GLADE_LWC_COMPILE: "1"`, `NO_COLOR: "1"`} {
		if !strings.Contains(job, marker) {
			t.Errorf("browser matrix missing %q", marker)
		}
	}
	for _, topology := range []struct {
		pkg, slug string
		count     int
	}{{"./internal/visualforce", "internal-visualforce", 4}, {"./internal/lwc/compile", "internal-lwc-compile", 8}} {
		if strings.Count(job, "- package: "+topology.pkg) != topology.count {
			t.Errorf("wrong runner count for %s", topology.pkg)
		}
		for index := 0; index < topology.count; index++ {
			marker := fmt.Sprintf("- package: %s\n            slug: %s\n            shard: %d", topology.pkg, topology.slug, index)
			if !strings.Contains(job, marker) {
				t.Errorf("missing exact browser matrix entry %q", marker)
			}
		}
	}
	if !strings.Contains(workflow, "scripts/ci-race-shard-aggregate.sh") || !strings.Contains(workflow, "needs: [plan, race-browser]") {
		t.Fatal("browser shards must require an exact union aggregate")
	}
	if strings.Count(workflow, "GLADE_LWC_PLAYWRIGHT_MODULE: ${{ github.workspace }}/lwcruntime/node_modules/playwright") != 2 {
		t.Fatal("both browser matrix and generic browser consumers must resolve the installed Playwright module")
	}
}
