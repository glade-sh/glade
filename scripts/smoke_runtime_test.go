package scripts

import (
	"os"
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

func TestSmokeAggregateBuildsRuntimeOnce(t *testing.T) {
	aggregate := readSmokeScript(t, "smoke.sh")

	build := regexp.MustCompile(`(?m)^CGO_ENABLED=1 go build -o "\$\{TMP\}/glade" ./cmd/glade$`)
	if got := len(build.FindAllStringIndex(aggregate, -1)); got != 1 {
		t.Errorf("smoke.sh direct runtime build count = %d, want 1", got)
	}
	for _, want := range []string{
		`scripts/smoke-runtime.sh "${GLADE}"`,
		`DIST_DIR="${TMP}/release-dist" VERSION=smoke scripts/release-build.sh`,
		`grep -q 'release artifact written'`,
		`grep -q 'release manifest written'`,
		`grep -q 'glade_smoke_'`,
		`grep -q '"parserSmoke": "passed"'`,
		`echo "smoke: ok"`,
	} {
		if !strings.Contains(aggregate, want) {
			t.Errorf("smoke.sh missing aggregate marker %q", want)
		}
	}
}
