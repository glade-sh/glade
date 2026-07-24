package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const requiredCISHA = "0123456789abcdef0123456789abcdef01234567"

func runRequiredCIFixture(t *testing.T, runs, jobs string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runsPath := filepath.Join(dir, "runs.json")
	jobsPath := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(runsPath, []byte(runs), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobsPath, []byte(jobs), 0o600); err != nil {
		t.Fatal(err)
	}
	gh := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$VERIFY_CI_CALLS"
case "$*" in
  *"/actions/workflows/ci.yml/runs"*) cat "$VERIFY_CI_RUNS" ;;
  *"/actions/runs/101/jobs"*) cat "$VERIFY_CI_JOBS" ;;
  *) echo "unexpected gh call: $*" >&2; exit 97 ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(gh), 0o700); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls")
	cmd := exec.Command("bash", "verify-required-ci.sh", "glade-sh/glade", requiredCISHA)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"VERIFY_CI_RUNS="+runsPath,
		"VERIFY_CI_JOBS="+jobsPath,
		"VERIFY_CI_CALLS="+calls,
	)
	out, err := cmd.CombinedOutput()
	callData, readErr := os.ReadFile(calls)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(out) + "\nCALLS\n" + string(callData), err
}

func TestVerifyRequiredCIFindsExactSuccessfulAuthority(t *testing.T) {
	runs := `{"workflow_runs":[
{"id":99,"head_sha":"ffffffffffffffffffffffffffffffffffffffff","conclusion":"success","event":"push","created_at":"2026-01-01T00:00:00Z","html_url":"https://example/99"},
{"id":100,"head_sha":"` + requiredCISHA + `","conclusion":"failure","event":"push","created_at":"2026-01-02T00:00:00Z","html_url":"https://example/100"},
{"id":101,"head_sha":"` + requiredCISHA + `","conclusion":"success","event":"push","created_at":"2026-01-03T00:00:00Z","html_url":"https://example/101"}
]}`
	jobs := `{"jobs":[{"id":501,"name":"Required CI","head_sha":"` + requiredCISHA + `","status":"completed","conclusion":"success","html_url":"https://example/job/501"}]}`
	out, err := runRequiredCIFixture(t, runs, jobs)
	if err != nil {
		t.Fatalf("verification failed: %v\n%s", err, out)
	}
	jsonPart := strings.Split(out, "\nCALLS\n")[0]
	var got map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &got); err != nil {
		t.Fatalf("invalid verification JSON: %v\n%s", err, out)
	}
	if got["schema_version"] != float64(1) || got["sha"] != requiredCISHA || got["run_id"] != float64(101) || got["required_ci_job_id"] != float64(501) || got["conclusion"] != "success" {
		t.Fatalf("unexpected verification: %#v", got)
	}
	for _, marker := range []string{"--method GET", "head_sha=" + requiredCISHA, "status=completed", "/actions/runs/101/jobs"} {
		if !strings.Contains(out, marker) {
			t.Errorf("API calls missing %q\n%s", marker, out)
		}
	}
}

func TestVerifyRequiredCIRejectsPullRequestAuthority(t *testing.T) {
	runs := `{"workflow_runs":[{"id":101,"head_sha":"` + requiredCISHA + `","conclusion":"success","event":"pull_request","created_at":"2026-01-03T00:00:00Z","html_url":"https://example/101"}]}`
	jobs := `{"jobs":[{"id":501,"name":"Required CI","head_sha":"` + requiredCISHA + `","status":"completed","conclusion":"success","html_url":"https://example/job/501"}]}`
	out, err := runRequiredCIFixture(t, runs, jobs)
	if err == nil || !strings.Contains(out, "no successful Required CI authority") {
		t.Fatalf("pull request authority accepted: err=%v\n%s", err, out)
	}
}

func TestVerifyRequiredCIRejectsMissingOrFailedAuthority(t *testing.T) {
	runs := `{"workflow_runs":[{"id":101,"head_sha":"` + requiredCISHA + `","conclusion":"success","event":"push","created_at":"2026-01-03T00:00:00Z","html_url":"https://example/101"}]}`
	cases := map[string]string{
		"missing":   `{"jobs":[{"id":1,"name":"vet","head_sha":"` + requiredCISHA + `","status":"completed","conclusion":"success"}]}`,
		"failed":    `{"jobs":[{"id":501,"name":"Required CI","head_sha":"` + requiredCISHA + `","status":"completed","conclusion":"failure"}]}`,
		"wrong sha": `{"jobs":[{"id":501,"name":"Required CI","head_sha":"ffffffffffffffffffffffffffffffffffffffff","status":"completed","conclusion":"success"}]}`,
	}
	for name, jobs := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := runRequiredCIFixture(t, runs, jobs)
			if err == nil || !strings.Contains(out, "no successful Required CI authority") {
				t.Fatalf("invalid authority accepted: err=%v\n%s", err, out)
			}
		})
	}
}

func TestReleaseTagAttestationWorkflowContract(t *testing.T) {
	ciData, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	ci := string(ciData)
	if !strings.Contains(ci, "push:\n    branches:") || strings.Contains(ci, "tags: ['v*']") {
		t.Fatal("CI must run for branch pushes and not tag pushes")
	}
	releaseData, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	release := string(releaseData)
	for _, marker := range []string{
		"required-ci-attestation:", "actions: read", "scripts/verify-required-ci.sh", "required-ci-attestation.json",
		"needs: required-ci-attestation", "attest-and-upload:", "needs: [prepare, build]", "Attest platform archive", "Attest platform SBOM",
		"Verify platform attestations", "gh attestation verify", "--predicate-type https://cyclonedx.org/bom",
		"Upload platform release assets", "publish:\n    needs: attest-and-upload",
	} {
		if !strings.Contains(release, marker) {
			t.Errorf("release workflow missing %q", marker)
		}
	}
	for _, step := range []string{"Attest platform archive", "Attest platform SBOM"} {
		start := strings.Index(release, "- name: "+step)
		if start < 0 {
			continue
		}
		end := strings.Index(release[start+1:], "\n      - name:")
		block := release[start:]
		if end >= 0 {
			block = release[start : start+1+end]
		}
		if strings.Contains(block, "continue-on-error") {
			t.Errorf("%s remains best-effort", step)
		}
	}
	verifyIndex := strings.Index(release, "- name: Verify platform attestations")
	uploadIndex := strings.Index(release, "- name: Upload platform release assets")
	if verifyIndex < 0 || uploadIndex < 0 || verifyIndex > uploadIndex {
		t.Fatal("platform assets can publish before attestation verification")
	}
}

func TestReleaseAttestationDocsAreFailClosed(t *testing.T) {
	paths := []string{
		filepath.Join("..", "SECURITY.md"),
		filepath.Join("..", "docs", "INSTALL.md"),
		filepath.Join("..", "docs", "SECURITY_TRUST.md"),
		filepath.Join("..", "site", "docs-src", "guide", "installation.md"),
		filepath.Join("..", "site", "docs-src", "guide", "security-trust.md"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(data))
		if !strings.Contains(text, "--predicate-type https://cyclonedx.org/bom") {
			t.Errorf("%s lacks CycloneDX attestation verification", path)
		}
		for _, stale := range []string{"best-effort attest", "when the repository host supports", "if no attestation", "does not publish an attestation"} {
			if strings.Contains(text, stale) {
				t.Errorf("%s retains fail-open wording %q", path, stale)
			}
		}
	}

	installData, err := os.ReadFile(filepath.Join("..", "docs", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(installData), "--predicate-type https://cyclonedx.org/bom"); got != 2 {
		t.Errorf("docs/INSTALL.md CycloneDX verification count = %d, want 2", got)
	}
}
