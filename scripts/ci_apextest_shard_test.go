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

const fixturePackage = "github.com/glade-sh/glade/internal/apextest"

func writeApexFixtureExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func runApexShardFixture(t *testing.T, index, discovery string, discoveryRC int, plan string, plannerRC int, events string, nativeRC int) (string, error, string) {
	t.Helper()
	return runApexShardFixtureWithDiscoveryStderr(t, index, discovery, "", discoveryRC, plan, plannerRC, events, nativeRC)
}

func runApexShardFixtureWithDiscoveryStderr(t *testing.T, index, discovery, discoveryStderr string, discoveryRC int, plan string, plannerRC int, events string, nativeRC int) (string, error, string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	artifacts := filepath.Join(dir, "artifacts")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	discoveryPath := filepath.Join(dir, "discovery.fixture")
	discoveryStderrPath := filepath.Join(dir, "discovery-stderr.fixture")
	eventsPath := filepath.Join(dir, "events.fixture")
	if err := os.WriteFile(discoveryPath, []byte(discovery), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(discoveryStderrPath, []byte(discoveryStderr), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eventsPath, []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	writeApexFixtureExecutable(t, filepath.Join(binDir, "go"), `#!/usr/bin/env bash
set -u
if [[ "$*" == "test -list ^Test ./internal/apextest" ]]; then
  cat "$FIXTURE_DISCOVERY"
  cat "$FIXTURE_DISCOVERY_STDERR" >&2
  exit "$FIXTURE_DISCOVERY_RC"
fi
if [[ "$1" == "test" && "$2" == "-json" ]]; then
  printf '%s\n' "$*" >> "$FIXTURE_CALLS"
  cat "$FIXTURE_EVENTS"
  exit "$FIXTURE_NATIVE_RC"
fi
echo "unexpected go invocation: $*" >&2
exit 97
`)
	writeApexFixtureExecutable(t, filepath.Join(binDir, "planner"), fmt.Sprintf(`#!/usr/bin/env bash
cat >/dev/null
cat <<'EOF'
%s
EOF
exit %d
`, plan, plannerRC))
	calls := filepath.Join(dir, "calls")
	cmd := exec.Command("bash", "ci-go-test.sh", "apex-shard", index)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CI_SHARD_PLANNER="+filepath.Join(binDir, "planner"),
		"CI_APEXTEST_ARTIFACT_DIR="+artifacts,
		"FIXTURE_DISCOVERY="+discoveryPath,
		"FIXTURE_DISCOVERY_STDERR="+discoveryStderrPath,
		fmt.Sprintf("FIXTURE_DISCOVERY_RC=%d", discoveryRC),
		"FIXTURE_EVENTS="+eventsPath,
		fmt.Sprintf("FIXTURE_NATIVE_RC=%d", nativeRC),
		"FIXTURE_CALLS="+calls,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err, artifacts
}

func validFixturePlan() string {
	return `{"version":1,"package":"` + fixturePackage + `","historyUsed":false,"shards":[{"index":0,"tests":["TestAlpha"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha)$"},{"index":1,"tests":["TestBeta"],"estimatedDurationMillis":0,"regex":"^(?:TestBeta)$"}]}`
}

func passEvent(name string) string {
	return `{"Time":"2026-01-01T00:00:00Z","Action":"run","Package":"` + fixturePackage + `","Test":"` + name + `"}` + "\n" +
		`{"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"` + fixturePackage + `","Test":"` + name + `","Elapsed":1}` + "\n" +
		`{"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"` + fixturePackage + `","Elapsed":1}` + "\n"
}

func TestApexShardRunsOneNativeJSONCommandAndWritesEvidence(t *testing.T) {
	out, err, artifacts := runApexShardFixture(t, "1", "TestAlpha\nTestBeta\nok  \t"+fixturePackage+"\n", 0, validFixturePlan(), 0, passEvent("TestBeta"), 0)
	if err != nil {
		t.Fatalf("apex shard failed: %v\n%s", err, out)
	}
	calls, err := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
	if err != nil {
		t.Fatal(err)
	}
	callText := string(calls)
	if strings.Count(callText, "test -json") != 1 || !strings.Contains(callText, `-timeout=30m -run ^(?:TestBeta)$ ./internal/apextest`) {
		t.Fatalf("native calls = %q", callText)
	}
	for _, name := range []string{"discovery-command.txt", "discovery.txt", "discovery-stderr.txt", "plan.json", "selected-shard.json", "events.json", "validation-summary.json"} {
		if _, err := os.Stat(filepath.Join(artifacts, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
	var summary struct {
		Valid bool `json:"valid"`
	}
	b, err := os.ReadFile(filepath.Join(artifacts, "validation-summary.json"))
	if err != nil || json.Unmarshal(b, &summary) != nil || !summary.Valid {
		t.Fatalf("validation summary = %s err=%v", b, err)
	}
}

func TestApexShardSeparatesVisibleDiscoveryStderrFromAuthoritativeNames(t *testing.T) {
	diagnostic := "go: downloading example.invalid/module v1.2.3\n"
	out, err, artifacts := runApexShardFixtureWithDiscoveryStderr(t, "0", "TestAlpha\nTestBeta\n", diagnostic, 0, validFixturePlan(), 0, passEvent("TestAlpha"), 0)
	if err != nil {
		t.Fatalf("download diagnostic contaminated discovery: %v\n%s", err, out)
	}
	if !strings.Contains(out, diagnostic) {
		t.Fatalf("discovery stderr was not visible: %s", out)
	}
	stderrEvidence, readErr := os.ReadFile(filepath.Join(artifacts, "discovery-stderr.txt"))
	if readErr != nil || string(stderrEvidence) != diagnostic {
		t.Fatalf("discovery stderr evidence = %q err=%v", stderrEvidence, readErr)
	}
	discoveryEvidence, readErr := os.ReadFile(filepath.Join(artifacts, "discovery.txt"))
	if readErr != nil || string(discoveryEvidence) != "TestAlpha\nTestBeta\n" {
		t.Fatalf("authoritative discovery = %q err=%v", discoveryEvidence, readErr)
	}
}

func TestApexShardPreservesDiscoveryExitAndRendersStderr(t *testing.T) {
	diagnostic := "go: module download failed\n"
	out, err, artifacts := runApexShardFixtureWithDiscoveryStderr(t, "0", "", diagnostic, 6, validFixturePlan(), 0, passEvent("TestAlpha"), 0)
	if err == nil {
		t.Fatal("expected discovery failure")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 6 {
		t.Fatalf("exit = %v, want 6; output=%s", err, out)
	}
	if !strings.Contains(out, diagnostic) {
		t.Fatalf("discovery failure stderr was not rendered: %s", out)
	}
	b, readErr := os.ReadFile(filepath.Join(artifacts, "discovery-stderr.txt"))
	if readErr != nil || string(b) != diagnostic {
		t.Fatalf("discovery stderr evidence = %q err=%v", b, readErr)
	}
}

func TestApexShardPreservesPartialDiscoveryStdoutStderrAndExit(t *testing.T) {
	partialStdout := "TestAlpha\npartial discovery diagnostic\n"
	diagnostic := "go: dependency resolution failed\n"
	out, err, artifacts := runApexShardFixtureWithDiscoveryStderr(t, "0", partialStdout, diagnostic, 8, validFixturePlan(), 0, passEvent("TestAlpha"), 0)
	if err == nil {
		t.Fatal("expected discovery failure")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 8 {
		t.Fatalf("exit = %v, want 8; output=%s", err, out)
	}
	for name, want := range map[string]string{
		"discovery-command.txt": partialStdout,
		"discovery-stderr.txt":  diagnostic,
		"discovery.txt":         "",
	} {
		b, readErr := os.ReadFile(filepath.Join(artifacts, name))
		if readErr != nil || string(b) != want {
			t.Errorf("%s = %q, want %q, err=%v", name, b, want, readErr)
		}
	}
}

func TestApexShardRejectsInvalidIndexBeforeDiscovery(t *testing.T) {
	for _, index := range []string{"", "-1", "2", "nope"} {
		out, err, artifacts := runApexShardFixture(t, index, "TestAlpha\nTestBeta\n", 0, validFixturePlan(), 0, passEvent("TestAlpha"), 0)
		if err == nil || !strings.Contains(out, "shard index must be 0 or 1") {
			t.Fatalf("index %q err=%v out=%s", index, err, out)
		}
		for _, artifact := range []string{"discovery-command.txt", "discovery.txt", "discovery-stderr.txt", "plan.json", "selected-shard.json", "events.json", "validation-summary.json"} {
			if _, statErr := os.Stat(filepath.Join(artifacts, artifact)); statErr != nil {
				t.Errorf("invalid index %q omitted artifact %s: %v", index, artifact, statErr)
			}
		}
	}
}

func TestApexShardFailsClosedOnDiscoveryAndPlannerFailures(t *testing.T) {
	cases := []struct {
		name, discovery, plan  string
		discoveryRC, plannerRC int
	}{
		{"discovery command", "", validFixturePlan(), 4, 0},
		{"empty discovery", "ok  \t" + fixturePackage + "\n", validFixturePlan(), 0, 0},
		{"duplicate discovery", "TestAlpha\nTestAlpha\n", validFixturePlan(), 0, 0},
		{"invalid discovery", "TestAlpha\nBenchmarkBeta\n", validFixturePlan(), 0, 0},
		{"planner command", "TestAlpha\nTestBeta\n", "", 0, 5},
		{"planner union", "TestAlpha\nTestBeta\n", strings.Replace(validFixturePlan(), "TestBeta", "TestExtra", 1), 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err, artifacts := runApexShardFixture(t, "0", tc.discovery, tc.discoveryRC, tc.plan, tc.plannerRC, passEvent("TestAlpha"), 0)
			if err == nil {
				t.Fatalf("expected failure: %s", out)
			}
			for _, artifact := range []string{"discovery-command.txt", "discovery.txt", "discovery-stderr.txt", "plan.json", "selected-shard.json", "events.json", "validation-summary.json"} {
				if _, statErr := os.Stat(filepath.Join(artifacts, artifact)); statErr != nil {
					t.Errorf("failure path omitted artifact %s: %v", artifact, statErr)
				}
			}
		})
	}
}

func TestApexShardPreservesNativeExitAndRendersOutput(t *testing.T) {
	events := `{"Action":"run","Package":"` + fixturePackage + `","Test":"TestAlpha"}` + "\n" +
		`{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"fixture failure evidence\n"}` + "\n" +
		`{"Action":"fail","Package":"` + fixturePackage + `","Test":"TestAlpha"}` + "\n"
	out, err, _ := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\n", 0, validFixturePlan(), 0, events, 7)
	if err == nil {
		t.Fatal("expected native failure")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("exit = %v, want 7; output=%s", err, out)
	}
	if !strings.Contains(out, "fixture failure evidence\n") {
		t.Fatalf("failure output was not rendered: %s", out)
	}
}

func TestApexShardRendersNonOutputActionDiagnosticsInEventOrder(t *testing.T) {
	events := `{"Action":"build-output","Package":"` + fixturePackage + `","Output":"compile diagnostic one\n"}` + "\n" +
		`{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"test diagnostic two\n"}` + "\n" +
		`{"Action":"fail","Package":"` + fixturePackage + `","Test":"TestAlpha"}` + "\n"
	out, err, _ := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\n", 0, validFixturePlan(), 0, events, 9)
	if err == nil {
		t.Fatal("expected native failure")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 9 {
		t.Fatalf("exit = %v, want 9; output=%s", err, out)
	}
	first := strings.LastIndex(out, "compile diagnostic one\n")
	second := strings.LastIndex(out, "test diagnostic two\n")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("diagnostics were not rendered readably in event order: %s", out)
	}
}

func TestApexShardRejectsResultSetMismatches(t *testing.T) {
	cases := map[string]string{
		"missing":      `{"Action":"pass","Package":"` + fixturePackage + `"}` + "\n",
		"duplicate":    passEvent("TestAlpha") + `{"Action":"pass","Package":"` + fixturePackage + `","Test":"TestAlpha"}` + "\n",
		"extra":        passEvent("TestAlpha") + `{"Action":"pass","Package":"` + fixturePackage + `","Test":"TestExtra"}` + "\n",
		"wrong":        `{"Action":"fail","Package":"` + fixturePackage + `","Test":"TestAlpha"}` + "\n",
		"invalid json": "not-json\n",
	}
	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			out, err, artifacts := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\n", 0, validFixturePlan(), 0, events, 0)
			if err == nil {
				t.Fatalf("expected validation failure: %s", out)
			}
			b, readErr := os.ReadFile(filepath.Join(artifacts, "validation-summary.json"))
			if readErr != nil || !strings.Contains(string(b), `"valid": false`) {
				t.Fatalf("summary=%s err=%v", b, readErr)
			}
		})
	}
}

func TestApexShardRendersOutputWhenResultValidationFails(t *testing.T) {
	events := `{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"validation failure evidence\n"}` + "\n" +
		`{"Action":"pass","Package":"` + fixturePackage + `"}` + "\n"
	out, err, _ := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\n", 0, validFixturePlan(), 0, events, 0)
	if err == nil {
		t.Fatal("expected result validation failure")
	}
	if !strings.Contains(out, "validation failure evidence\n") {
		t.Fatalf("validation failure Output record was not rendered: %s", out)
	}
}
