package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeResourceTime(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "time")
	contents := `#!/usr/bin/env bash
set -uo pipefail
output=""
quiet=0
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -o) output="$2"; shift 2 ;;
    -f) shift 2 ;;
    --quiet) quiet=1; shift ;;
    --) shift; break ;;
    *) echo "unexpected timer argument: $1" >&2; exit 98 ;;
  esac
done
set +e
"$@"
rc="$?"
set -e
printf '{"schema_version":1,"lane":"%s","elapsed_seconds":1.25,"user_seconds":0.75,"system_seconds":0.25,"max_rss_kb":4096}\n' "$CI_RESOURCE_LABEL" >"$output"
if [[ "$rc" -ne 0 && "$quiet" -ne 1 ]]; then
  printf 'Command exited with non-zero status %s\n' "$rc" >>"$output"
fi
exit "$rc"
`
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func runResourceFixture(t *testing.T, command string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	output := filepath.Join(dir, "nested", "lane.json")
	cmd := exec.Command("bash", "ci-resource-run.sh", output, "remaining-go", "--", "bash", "-c", command)
	cmd.Env = append(os.Environ(), "CI_RESOURCE_TIME_COMMAND="+writeFakeResourceTime(t, dir))
	combined, err := cmd.CombinedOutput()
	data, readErr := os.ReadFile(output)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(combined), string(data), err
}

func TestCIResourceRunWritesValidatedTelemetry(t *testing.T) {
	stdout, data, err := runResourceFixture(t, "printf 'authority ran\\n'")
	if err != nil {
		t.Fatalf("resource wrapper failed: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "authority ran") || !strings.Contains(stdout, "[ci] resource usage remaining-go") {
		t.Fatalf("wrapper output lacks command or telemetry evidence:\n%s", stdout)
	}
	var got struct {
		SchemaVersion  int     `json:"schema_version"`
		Lane           string  `json:"lane"`
		ElapsedSeconds float64 `json:"elapsed_seconds"`
		MaxRSSKB       int64   `json:"max_rss_kb"`
		ExitStatus     int     `json:"exit_status"`
	}
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("invalid telemetry JSON: %v\n%s", err, data)
	}
	if got.SchemaVersion != 1 || got.Lane != "remaining-go" || got.ElapsedSeconds != 1.25 || got.MaxRSSKB != 4096 || got.ExitStatus != 0 {
		t.Fatalf("unexpected telemetry: %+v", got)
	}
}

func TestCIResourceRunPreservesCommandFailure(t *testing.T) {
	stdout, data, err := runResourceFixture(t, "exit 23")
	if err == nil {
		t.Fatalf("failing authority accepted:\n%s", stdout)
	}
	if exit := err.(*exec.ExitError).ExitCode(); exit != 23 {
		t.Fatalf("wrapper exit = %d, want 23\n%s", exit, stdout)
	}
	if !strings.Contains(data, `"exit_status": 23`) {
		t.Fatalf("failure telemetry missing native status: %s", data)
	}
	if !json.Valid([]byte(data)) {
		t.Fatalf("failure telemetry is not standalone JSON: %s", data)
	}
}

func TestCIResourceRunRejectsInvalidLabelAndTelemetry(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "lane.json")
	badLabel := exec.Command("bash", "ci-resource-run.sh", output, "bad lane", "--", "true")
	if out, err := badLabel.CombinedOutput(); err == nil || !strings.Contains(string(out), "invalid resource lane") {
		t.Fatalf("invalid label accepted: err=%v out=%s", err, out)
	}

	badTime := filepath.Join(dir, "bad-time")
	if err := os.WriteFile(badTime, []byte("#!/usr/bin/env bash\nprintf '{not-json}\\n' >\"$2\"\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-resource-run.sh", output, "test", "--", "true")
	cmd.Env = append(os.Environ(), "CI_RESOURCE_TIME_COMMAND="+badTime)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "resource telemetry rejected") {
		t.Fatalf("invalid telemetry accepted: err=%v out=%s", err, out)
	}

	badSchemaTime := filepath.Join(dir, "bad-schema-time")
	badSchema := "#!/usr/bin/env bash\noutput=\"$2\"\n" +
		"printf '{\"schema_version\":true,\"lane\":\"test\",\"elapsed_seconds\":1,\"user_seconds\":1,\"system_seconds\":1,\"max_rss_kb\":1}\\n' >\"$output\"\n"
	if err := os.WriteFile(badSchemaTime, []byte(badSchema), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", "ci-resource-run.sh", output, "test", "--", "true")
	cmd.Env = append(os.Environ(), "CI_RESOURCE_TIME_COMMAND="+badSchemaTime)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "resource telemetry rejected") {
		t.Fatalf("boolean schema version accepted: err=%v out=%s", err, out)
	}
}

func TestCIResourceRunPreservesNativeFailureWhenTelemetryIsMalformed(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "lane.json")
	badTime := filepath.Join(dir, "bad-time")
	contents := `#!/usr/bin/env bash
output=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -o) output="$2"; shift 2 ;;
    -f) shift 2 ;;
    --quiet) shift ;;
    --) shift; break ;;
    *) exit 98 ;;
  esac
done
set +e
"$@"
rc="$?"
printf '{not-json}\n' >"$output"
exit "$rc"
`
	if err := os.WriteFile(badTime, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-resource-run.sh", output, "test", "--", "bash", "-c", "exit 23")
	cmd.Env = append(os.Environ(), "CI_RESOURCE_TIME_COMMAND="+badTime)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("malformed failure evidence accepted: %s", out)
	}
	if exit := err.(*exec.ExitError).ExitCode(); exit != 23 {
		t.Fatalf("malformed telemetry masked native exit: got %d, want 23\n%s", exit, out)
	}
	if !strings.Contains(string(out), "resource telemetry rejected") {
		t.Fatalf("malformed telemetry was not reported: %s", out)
	}
}

func TestCIResourceRunRecordsWrapperStatusForSignaledCommand(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "lane.json")
	signalTime := filepath.Join(dir, "signal-time")
	contents := `#!/usr/bin/env bash
output="$2"
printf '{"schema_version":1,"lane":"test","elapsed_seconds":1,"user_seconds":0,"system_seconds":0,"max_rss_kb":1024}\n' >"$output"
exit 143
`
	if err := os.WriteFile(signalTime, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-resource-run.sh", output, "test", "--", "bash", "-c", "kill -TERM $$")
	cmd.Env = append(os.Environ(), "CI_RESOURCE_TIME_COMMAND="+signalTime)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("signal termination accepted: %s", out)
	}
	if exit := err.(*exec.ExitError).ExitCode(); exit != 143 {
		t.Fatalf("wrapper exit = %d, want 143\n%s", exit, out)
	}
	data, readErr := os.ReadFile(output)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !json.Valid(data) || !strings.Contains(string(data), `"exit_status": 143`) {
		t.Fatalf("signal telemetry does not contain wrapper status: %s", data)
	}
}

func TestCIResourceTelemetryCoversAuthoritativeLanes(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	want := map[string][]string{
		"gladecli":              {"resource-gladecli.json", "scripts/ci-go-test.sh lane gladecli"},
		"node-integration":      {"resource-usage.json", "scripts/ci-go-test.sh node-integration"},
		"sema":                  {"resource-usage.json", "scripts/ci-go-test.sh sema-shard"},
		"sema-full":             {"resource-usage.json", "scripts/ci-go-test.sh sema-full"},
		"server-and-playground": {"resource-server-and-playground.json", "scripts/ci-go-test.sh lane server-and-playground"},
		"test":                  {"resource-repoguard.json", "resource-remaining-go.json", "scripts/ci-go-test.sh lane repoguard", "scripts/ci-go-test.sh lane remaining-go"},
		"apextest":              {"resource-usage.json", "scripts/ci-go-test.sh apex-shard"},
	}
	for job, markers := range want {
		for _, marker := range markers {
			if !strings.Contains(jobs[job], marker) {
				t.Errorf("%s lacks resource marker %q", job, marker)
			}
		}
		if !strings.Contains(jobs[job], "scripts/ci-resource-run.sh") {
			t.Errorf("%s does not use resource wrapper", job)
		}
	}
	if got := strings.Count(workflow, "scripts/ci-resource-run.sh"); got != 8 {
		t.Fatalf("resource wrapper invocation count = %d, want 8", got)
	}
	for _, forbidden := range []string{"runs-on: ubuntu-latest-", "GOMAXPROCS: \"4\"", "GOMAXPROCS: \"8\""} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("resource telemetry changed runner policy with %q", forbidden)
		}
	}
}
