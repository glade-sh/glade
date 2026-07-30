package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readSmokeScript(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func TestRuntimeSmokeUsesProvidedBinary(t *testing.T) {
	runtimeSmoke := readSmokeScript(t, "smoke-runtime.sh")

	for _, want := range []string{
		`if [[ "$#" -ne 1 ]]`,
		`exit 2`,
		`if [[ ! -e "${GLADE}" ]]`,
		`if [[ ! -f "${GLADE}" || ! -x "${GLADE}" ]]`,
		`GLADE="${PWD}/${GLADE}"`,
		`cd "${ROOT}"`,
	} {
		if !strings.Contains(runtimeSmoke, want) {
			t.Errorf("smoke-runtime.sh missing executable validation marker %q", want)
		}
	}
	if strings.Index(runtimeSmoke, `GLADE="${PWD}/${GLADE}"`) > strings.Index(runtimeSmoke, `cd "${ROOT}"`) {
		t.Error("smoke-runtime.sh must resolve a relative executable before changing to repository root")
	}
	if strings.Contains(runtimeSmoke, "go build") || strings.Contains(runtimeSmoke, "release-build.sh") {
		t.Error("smoke-runtime.sh must not build or package a replacement executable")
	}

	invocation := regexp.MustCompile(`(?m)^"\$\{GLADE\}"(?:\s|$)`)
	if got := len(invocation.FindAllStringIndex(runtimeSmoke, -1)); got != 12 {
		t.Errorf("smoke-runtime.sh has %d Glade invocations through GLADE, want 12", got)
	}
	for _, line := range strings.Split(runtimeSmoke, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "glade ") || strings.HasPrefix(trimmed, "./glade ") {
			t.Errorf("smoke-runtime.sh bypasses GLADE: %s", trimmed)
		}
	}
}

func TestRuntimeSmokePreservesCoverage(t *testing.T) {
	runtimeSmoke := readSmokeScript(t, "smoke-runtime.sh")

	for _, want := range []string{
		`"${GLADE}" version >/dev/null`,
		`Sample.cls`,
		`SampleTest.cls`,
		`"${GLADE}" parse`,
		`grep -q '"name": "Sample"'`,
		`"${GLADE}" check`,
		`grep -q '"diagnostics": 0'`,
		`"${GLADE}" exec --trace "${TMP}/trace.json"`,
		`grep -q 'x=2'`,
		`"${GLADE}" profile analyze "${TMP}/trace.json" --json`,
		`grep -q '"events"'`,
		`"${GLADE}" test`,
		`grep -q '"passed": 1'`,
		`"${GLADE}" db seed`,
		`grep -q '"Account": 1'`,
		`"${GLADE}" db inspect`,
		`grep -q 'Account: 1'`,
		`"${GLADE}" db ui`,
		`grep -q 'Glade Local Data'`,
		`"${GLADE}" playground`,
		`grep -q 'http://127.0.0.1:1789/playground/'`,
		`"${GLADE}" lsp`,
		`grep -q 'textDocument/publishDiagnostics'`,
		`"${GLADE}" server`,
		`curl -fsS "http://${ADDR}/services/data"`,
		`grep -q 'v65.0'`,
		`kill "${SERVER_PID}"`,
		`wait "${SERVER_PID}"`,
		`rm -rf "${TMP}"`,
		`trap cleanup EXIT`,
	} {
		if !strings.Contains(runtimeSmoke, want) {
			t.Errorf("smoke-runtime.sh missing runtime coverage marker %q", want)
		}
	}
	if got := strings.Count(runtimeSmoke, `echo "runtime smoke: ok"`); got != 1 {
		t.Errorf("smoke-runtime.sh success marker count = %d, want 1", got)
	}
}

func TestSmokeAggregateUsesOneVerifiedPackagedBinary(t *testing.T) {
	aggregate := readSmokeScript(t, "smoke.sh")

	for _, want := range []string{
		`scripts/smoke-distribution.sh`,
		`echo "smoke: ok"`,
	} {
		if !strings.Contains(aggregate, want) {
			t.Errorf("smoke.sh missing aggregate marker %q", want)
		}
	}
	if got := strings.Count(aggregate, `scripts/smoke-distribution.sh`); got != 1 {
		t.Errorf("smoke.sh distribution helper call count = %d, want 1", got)
	}
	if got := strings.Count(aggregate, `smoke-runtime.sh`); got != 0 {
		t.Errorf("smoke.sh direct runtime smoke count = %d, want 0; distribution must runtime-smoke the verified extracted binary", got)
	}
	for _, forbidden := range []string{"go build", "release-build.sh", "release-manifest.json", "parserSmoke", "release artifact written"} {
		if strings.Contains(aggregate, forbidden) {
			t.Errorf("smoke.sh retains inline distribution implementation marker %q", forbidden)
		}
	}
}

func TestDistributionSmokeUsesProvidedDistributionWithoutRebuild(t *testing.T) {
	root, runtimeLog := distributionFixture(t, false)
	dist := filepath.Join(root, "prebuilt-dist")
	build := exec.Command(filepath.Join(root, "scripts", "release-build.sh"))
	build.Dir = root
	build.Env = append(os.Environ(), "DIST_DIR="+dist, "VERSION=smoke")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("prepare prebuilt distribution: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "calls.log"), nil, 0o600); err != nil {
		t.Fatalf("clear fixture calls: %v", err)
	}

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "DIST_DIR="+dist)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("distribution smoke with prebuilt dist failed: %v\n%s", err, out)
	}
	counts, err := os.ReadFile(filepath.Join(root, "calls.log"))
	if err != nil {
		t.Fatalf("read calls: %v", err)
	}
	if got := strings.Count(string(counts), "release\n"); got != 0 {
		t.Fatalf("provided distribution triggered release build %d times", got)
	}
	if got := strings.Count(string(counts), "runtime\n"); got != 1 {
		t.Fatalf("provided distribution runtime smoke count = %d, want 1", got)
	}
	invoked, err := os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	if got := strings.TrimSpace(string(invoked)); !filepath.IsAbs(got) || filepath.Base(got) != "glade" || strings.Contains(got, "decoy") {
		t.Fatalf("runtime received %q, want the verified extracted glade binary", got)
	}
}

func TestDistributionSmokeUsesProvidedNonSmokeDistributionWithoutRebuild(t *testing.T) {
	const version = "v9.8.7"
	root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{version: version})
	dist := filepath.Join(root, "prebuilt-dist")
	build := exec.Command(filepath.Join(root, "scripts", "release-build.sh"))
	build.Dir = root
	build.Env = append(os.Environ(), "DIST_DIR="+dist, "VERSION="+version)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("prepare non-smoke prebuilt distribution: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "calls.log"), nil, 0o600); err != nil {
		t.Fatalf("clear fixture calls: %v", err)
	}

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "DIST_DIR="+dist)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("distribution smoke with non-smoke prebuilt dist failed: %v\n%s", err, out)
	}
	counts, err := os.ReadFile(filepath.Join(root, "calls.log"))
	if err != nil {
		t.Fatalf("read calls: %v", err)
	}
	if got := strings.Count(string(counts), "release\n"); got != 0 {
		t.Fatalf("provided non-smoke distribution triggered release build %d times", got)
	}
	if got := strings.Count(string(counts), "runtime\n"); got != 1 {
		t.Fatalf("provided non-smoke distribution runtime smoke count = %d, want 1", got)
	}
	if _, err := os.Stat(runtimeLog); err != nil {
		t.Fatalf("runtime smoke was not invoked: %v", err)
	}
}

func TestDistributionSmokeRejectsManifestDoctorClaimWithoutBinaryProof(t *testing.T) {
	root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{doctorFails: true})

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("distribution smoke accepted manifest doctor marker without binary proof:\n%s", out)
	}
	if !strings.Contains(string(out), "release binary doctor parser verification failed") {
		t.Fatalf("doctor proof rejection was not reported:\n%s", out)
	}
	if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
		t.Fatalf("runtime was invoked before doctor proof rejection: %v", err)
	}
}

func TestDistributionSmokeRejectsFailedDoctorProcessWithPassingJSON(t *testing.T) {
	root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{doctorExitCode: 1})

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("distribution smoke accepted a failed doctor process with passing JSON:\n%s", out)
	}
	if !strings.Contains(string(out), "release binary doctor parser verification failed") {
		t.Fatalf("failed doctor process rejection was not reported:\n%s", out)
	}
	if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
		t.Fatalf("runtime was invoked after failed doctor process: %v", err)
	}
}

func TestDistributionSmokeRejectsMalformedDoctorWithParserMarkerBeforeRuntime(t *testing.T) {
	root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{doctorOutput: `not JSON but "parserOK": true`})

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("distribution smoke accepted malformed doctor output with a parser marker:\n%s", out)
	}
	if !strings.Contains(string(out), "release binary doctor parser verification failed") {
		t.Fatalf("malformed doctor rejection was not reported:\n%s", out)
	}
	if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
		t.Fatalf("runtime was invoked before malformed doctor rejection: %v", err)
	}
}
