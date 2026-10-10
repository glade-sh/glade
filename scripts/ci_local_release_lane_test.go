package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type localReleaseLaneFixture struct {
	plan      string
	discovery string
	events    string
	nativeRC  int
	env       []string
}

func runLocalReleaseLaneFixture(t *testing.T, args []string, fixture localReleaseLaneFixture) (string, error, string, string) {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	var metadata, rows strings.Builder
	for lane, packages := range manifest.Lanes {
		for _, pkg := range packages {
			status := "has-tests"
			if strings.HasSuffix(pkg, "/lwcruntime/embed") {
				status = "no-tests"
			}
			fmt.Fprintf(&metadata, "%s\t%s\n", pkg, status)
			fmt.Fprintf(&rows, "%s\t./%s\n", lane, strings.TrimPrefix(pkg, "github.com/glade-sh/glade/"))
		}
	}
	plan := fixture.plan
	if plan == "" {
		plan = validFixturePlan()
	}
	discovery := fixture.discovery
	if discovery == "" {
		discovery = "TestAlpha\nTestBeta\nok  " + fixturePackage + "\n"
	}
	for name, contents := range map[string]string{
		"metadata": metadata.String(), "rows": rows.String(), "events": fixture.events, "discovery": discovery,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goScript := `#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then cat "$FIXTURE_DIR/metadata"; exit 0; fi
if [[ "$1" == "run" ]]; then cat "$FIXTURE_DIR/rows"; exit 0; fi
if [[ "$*" == "test -list ^Test ./internal/apextest" ]]; then
  cat "$FIXTURE_DIR/discovery"
  exit 0
fi
printf '%s\n' "$*" >> "$FIXTURE_DIR/calls"
printf 'GOMAXPROCS=%s GOFLAGS=%s\n' "${GOMAXPROCS-unset}" "${GOFLAGS-unset}" >> "$FIXTURE_DIR/environment"
if [[ "$1" == "vet" ]]; then exit "$FIXTURE_NATIVE_RC"; fi
if [[ "$1" != "test" || "$2" != "-json" ]]; then exit 91; fi
if [[ -s "$FIXTURE_DIR/events" ]]; then cat "$FIXTURE_DIR/events"; exit "$FIXTURE_NATIVE_RC"; fi
for argument in "$@"; do
  if [[ "$argument" == ./* ]]; then
    package="github.com/glade-sh/glade/${argument#./}"
    if [[ "$argument" == "./internal/apextest" ]]; then
      if [[ "$*" == *"TestAlpha"* ]]; then test=TestAlpha; else test=TestBeta; fi
      printf '{"Action":"pass","Package":"%s","Test":"%s"}\n' "$package" "$test"
    fi
    action=pass
    if [[ "$argument" == "./internal/lwcruntime/embed" ]]; then action=skip; fi
    printf '{"Action":"%s","Package":"%s"}\n' "$action" "$package"
  fi
done
exit "$FIXTURE_NATIVE_RC"
`
	writeApexFixtureExecutable(t, filepath.Join(dir, "go"), goScript)
	writeApexFixtureExecutable(t, filepath.Join(dir, "planner"), "#!/usr/bin/env bash\ncat <<'EOF'\n"+plan+"\nEOF\n")
	writeApexFixtureExecutable(t, filepath.Join(dir, "testlog"), "#!/usr/bin/env bash\ncat >/dev/null\n")
	cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOMAXPROCS=") && !strings.HasPrefix(entry, "GOFLAGS=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	artifacts := filepath.Join(dir, "artifacts")
	cmd.Env = append(cmd.Env,
		"PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE_DIR="+dir,
		fmt.Sprintf("FIXTURE_NATIVE_RC=%d", fixture.nativeRC),
		"CI_GO_COMMAND="+filepath.Join(dir, "go"), "CI_SHARD_PLANNER="+filepath.Join(dir, "planner"),
		"CI_TESTLOG_RENDERER="+filepath.Join(dir, "testlog"), "CI_LOCAL_RELEASE_ARTIFACT_DIR="+artifacts,
		"CI_LOCAL_RELEASE_METRICS=0", "LOCAL_GO_TEST_JOBS=1", "CI_GO_TEST_HEARTBEAT_SECONDS=1",
	)
	cmd.Env = append(cmd.Env, fixture.env...)
	out, cmdErr := cmd.CombinedOutput()
	calls, readErr := os.ReadFile(filepath.Join(dir, "calls"))
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(out), cmdErr, string(calls), artifacts
}

func TestCILocalReleaseLaneRunsOnlyItsOwnedPackages(t *testing.T) {
	data, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		lane, packageLane, timeout string
	}{
		{"guard", "repoguard", "30m"}, {"cli-ui", "gladecli", "30m"}, {"sema", "sema", "30m"},
		{"apex-0", "apextest", "90m"}, {"apex-1", "apextest", "90m"},
		{"server-playground", "server-and-playground", "30m"}, {"remaining", "remaining-go", "70m"},
	} {
		t.Run(tc.lane, func(t *testing.T) {
			out, err, calls, artifacts := runLocalReleaseLaneFixture(t, []string{"local-release-lane", tc.lane}, localReleaseLaneFixture{})
			if err != nil {
				t.Fatalf("lane failed: %v\n%s", err, out)
			}
			if strings.Count(strings.TrimSpace(calls), "\n") != 0 || !strings.HasPrefix(calls, "test -json -vet=off -count=1 -timeout="+tc.timeout) {
				t.Fatalf("lane must execute one uncached JSON command: %s", calls)
			}
			var got, want []string
			for _, argument := range strings.Fields(calls) {
				if strings.HasPrefix(argument, "./") {
					got = append(got, argument)
				}
				if strings.HasPrefix(argument, "-p=") || strings.HasPrefix(argument, "-parallel=") || argument == "-skip" {
					t.Errorf("lane overrides environment limits or skips tests: %s", calls)
				}
			}
			for _, pkg := range manifest.Lanes[tc.packageLane] {
				want = append(want, "./"+strings.TrimPrefix(pkg, "github.com/glade-sh/glade/"))
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("lane packages = %v, want %v", got, want)
			}
			for _, name := range []string{"events.json", "package-summary.json"} {
				data, err := os.ReadFile(filepath.Join(artifacts, tc.lane, name))
				if err != nil || (name == "package-summary.json" && !strings.Contains(string(data), `"valid": true`)) {
					t.Fatalf("lane artifact %s: %v\n%s", name, err, data)
				}
			}
			if strings.HasPrefix(tc.lane, "apex-") {
				wantTest := "TestAlpha"
				if tc.lane == "apex-1" {
					wantTest = "TestBeta"
				}
				if !strings.Contains(calls, "-run ^(?:"+wantTest+")$") {
					t.Fatalf("lane must run its existing shard selection: %s", calls)
				}
				data, err := os.ReadFile(filepath.Join(artifacts, tc.lane, "validation-summary.json"))
				if err != nil || !strings.Contains(string(data), `"valid": true`) {
					t.Fatalf("shard summary: %v\n%s", err, data)
				}
			}
		})
	}
}

func TestCILocalReleaseLaneInheritsConcurrencyEnvironment(t *testing.T) {
	for _, env := range [][]string{nil, {"GOMAXPROCS=3", "GOFLAGS=-p=2"}} {
		out, err, _, artifacts := runLocalReleaseLaneFixture(t, []string{"local-release-lane", "guard"}, localReleaseLaneFixture{env: env})
		if err != nil {
			t.Fatalf("lane failed: %v\n%s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "environment"))
		want := "GOMAXPROCS=unset GOFLAGS=unset\n"
		if env != nil {
			want = "GOMAXPROCS=3 GOFLAGS=-p=2\n"
		}
		if err != nil || string(data) != want {
			t.Fatalf("inherited environment = %q / %v, want %q", data, err, want)
		}
	}
}

func TestCILocalReleaseLaneVetAndUsage(t *testing.T) {
	out, err, calls, _ := runLocalReleaseLaneFixture(t, []string{"local-release-lane", "vet"}, localReleaseLaneFixture{})
	if err != nil || calls != "vet ./...\n" {
		t.Fatalf("vet lane = %q / %v\n%s", calls, err, out)
	}
	for _, args := range [][]string{{"local-release-lane"}, {"local-release-lane", "unknown"}, {"local-release-lane", "apex-2"}, {"local-release-lane", "guard", "extra"}} {
		out, err, calls, _ := runLocalReleaseLaneFixture(t, args, localReleaseLaneFixture{})
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 || calls != "" || !strings.Contains(out, "usage: scripts/ci-go-test.sh local-release-lane") {
			t.Fatalf("invalid args %v: %v, calls %q\n%s", args, err, calls, out)
		}
	}
}

func TestCILocalReleaseLanePreservesNativeFailureAndRejectsIncompleteResults(t *testing.T) {
	for _, lane := range []string{"guard", "apex-0"} {
		for _, tc := range []struct {
			name     string
			fixture  localReleaseLaneFixture
			wantCode int
		}{
			{"native failure", localReleaseLaneFixture{nativeRC: 23}, 23},
			{"missing results", localReleaseLaneFixture{events: "\n"}, 1},
			{"skipped subtest", localReleaseLaneFixture{events: passEvent("TestAlpha") + `{"Action":"skip","Package":"` + fixturePackage + `","Test":"TestAlpha/hidden"}` + "\n"}, 1},
		} {
			t.Run(lane+"/"+tc.name, func(t *testing.T) {
				out, err, _, _ := runLocalReleaseLaneFixture(t, []string{"local-release-lane", lane}, tc.fixture)
				exitErr, ok := err.(*exec.ExitError)
				if !ok || exitErr.ExitCode() != tc.wantCode {
					t.Fatalf("lane exit = %v, want %d\n%s", err, tc.wantCode, out)
				}
			})
		}
	}
}

func TestCILocalReleaseLaneMetricsIncludeApexShards(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("local-release resource metrics use macOS /usr/bin/time -l")
	}
	for _, lane := range []string{"guard", "apex-0"} {
		out, err, _, artifacts := runLocalReleaseLaneFixture(t, []string{"local-release-lane", lane}, localReleaseLaneFixture{env: []string{"CI_LOCAL_RELEASE_METRICS=1"}})
		if err != nil {
			t.Fatalf("metrics lane failed: %v\n%s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(artifacts, lane, "events.time.txt"))
		if err != nil || !strings.Contains(string(data), "maximum resident set size") {
			t.Fatalf("metrics missing: %v\n%s", err, data)
		}
	}
}

func TestCILocalReleaseApexLaneKeepsExplicitDataWeaveDeferrals(t *testing.T) {
	const name = "TestRunDataWeaveSourceSourceNames"
	events := `{"Action":"output","Package":"` + fixturePackage + `","Test":"` + name + `","Output":"    dataweave_test.go:1: requires explicitly installed DataWeave toolchain\n"}` + "\n" +
		`{"Action":"skip","Package":"` + fixturePackage + `","Test":"` + name + `"}` + "\n" +
		`{"Action":"pass","Package":"` + fixturePackage + `"}` + "\n"
	for _, configured := range []bool{false, true} {
		home := ""
		if configured {
			home = "/configured"
		}
		out, err, _, artifacts := runLocalReleaseLaneFixture(t, []string{"local-release-lane", "apex-0"}, localReleaseLaneFixture{
			plan:      strings.ReplaceAll(validFixturePlan(), "TestAlpha", name),
			discovery: name + "\nTestBeta\nok  " + fixturePackage + "\n",
			events:    events, env: []string{"GLADE_DATAWEAVE_APEX_TEST_HOME=" + home},
		})
		if configured {
			if err == nil || !strings.Contains(out, "skipped test results") {
				t.Fatalf("configured DataWeave skip accepted: %v\n%s", err, out)
			}
			continue
		}
		if err != nil {
			t.Fatalf("explicit DataWeave deferral failed: %v\n%s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(artifacts, "apex-0", "validation-summary.json"))
		if err != nil || !strings.Contains(string(data), `"valid": true`) || !strings.Contains(string(data), name) || !strings.Contains(string(data), `"deferred"`) {
			t.Fatalf("deferral missing from shard summary: %v\n%s", err, data)
		}
	}
}
