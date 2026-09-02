package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testGladeSHA = "1111111111111111111111111111111111111111"
	testToolsSHA = "2222222222222222222222222222222222222222"
	testDigest   = "3333333333333333333333333333333333333333333333333333333333333333"
)

func TestVerifySalesforceCheckAcceptsOneExactAuthority(t *testing.T) {
	root := makeSalesforceCheckFixture(t)
	response := fmt.Sprintf(`{"total_count":1,"check_runs":[{"id":42,"name":"Salesforce Correctness","head_sha":"%s","status":"completed","conclusion":"success","external_id":"salesforce-release-authority/v1;tools_sha=%s;run_id=123;run_attempt=2;receipt_sha256=%s","html_url":"https://github.com/glade-sh/glade/runs/42","app":{"id":4101915}}]}`, testGladeSHA, testToolsSHA, testDigest)
	out, err := runSalesforceCheckFixture(root, response, testGladeSHA, testToolsSHA)
	if err != nil {
		t.Fatalf("verify exact authority: %v\n%s", err, out)
	}
	for _, want := range []string{`"schemaVersion":1`, `"gladeSHA":"` + testGladeSHA + `"`, `"toolsSHA":"` + testToolsSHA + `"`, `"workflowRunID":123`, `"workflowRunAttempt":2`, `"receiptSHA256":"` + testDigest + `"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("evidence missing %s\n%s", want, out)
		}
	}
	invocation, err := os.ReadFile(filepath.Join(root, "gh.args"))
	if err != nil {
		t.Fatal(err)
	}
	wantEndpoint := "repos/glade-sh/glade/commits/" + testGladeSHA + "/check-runs?per_page=100&filter=latest&check_name=Salesforce%20Correctness"
	if !strings.Contains(string(invocation), wantEndpoint) {
		t.Fatalf("check-runs request does not filter by exact check name: %s", invocation)
	}
}

func TestVerifySalesforceCheckRejectsUntrustedOrAmbiguousResults(t *testing.T) {
	base := fmt.Sprintf(`{"id":42,"name":"Salesforce Correctness","head_sha":"%s","status":"completed","conclusion":"success","external_id":"salesforce-release-authority/v1;tools_sha=%s;run_id=123;run_attempt=2;receipt_sha256=%s","html_url":"https://github.com/glade-sh/glade/runs/42","app":{"id":4101915}}`, testGladeSHA, testToolsSHA, testDigest)
	cases := map[string]string{
		"wrong glade SHA": strings.Replace(base, testGladeSHA, strings.Repeat("a", 40), 1),
		"wrong tools SHA": strings.Replace(base, testToolsSHA, strings.Repeat("b", 40), 1),
		"wrong app":       strings.Replace(base, "4101915", "15368", 1),
		"non pass":        strings.Replace(base, `"conclusion":"success"`, `"conclusion":"failure"`, 1),
		"malformed id":    strings.Replace(base, "run_attempt=2", "run_attempt=02", 1),
		"duplicate JSON":  strings.Replace(base, `"id":42`, `"id":42,"id":43`, 1),
		"ambiguous":       base + "," + base,
	}
	for name, checkRuns := range cases {
		t.Run(name, func(t *testing.T) {
			root := makeSalesforceCheckFixture(t)
			count := 1
			if name == "ambiguous" {
				count = 2
			}
			response := fmt.Sprintf(`{"total_count":%d,"check_runs":[%s]}`, count, checkRuns)
			if out, err := runSalesforceCheckFixture(root, response, testGladeSHA, testToolsSHA); err == nil {
				t.Fatalf("hostile response accepted\n%s", out)
			}
		})
	}
}

func TestVerifySalesforceCheckRejectsPagination(t *testing.T) {
	root := makeSalesforceCheckFixture(t)
	if out, err := runSalesforceCheckFixture(root, `{"total_count":101,"check_runs":[]}`, testGladeSHA, testToolsSHA); err == nil {
		t.Fatalf("paginated response accepted\n%s", out)
	}
}

func TestVerifySalesforceCheckReadsToolsSHAFromAnnotatedTag(t *testing.T) {
	root := makeSalesforceCheckFixture(t)
	runGitSF(t, root, "init", "-q")
	runGitSF(t, root, "config", "user.name", "Test")
	runGitSF(t, root, "config", "user.email", "test@example.com")
	runGitSF(t, root, "add", ".")
	runGitSF(t, root, "commit", "-qm", "fixture")
	runGitSF(t, root, "tag", "-a", "v1.2.3", "-m", "Release v1.2.3\n\nGlade-Tools-SHA: "+testToolsSHA)

	cmd := exec.Command(filepath.Join(root, "scripts", "verify-salesforce-check.sh"), "--tag-tools-sha", "v1.2.3")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != testToolsSHA {
		t.Fatalf("parse annotated tag: %v\n%s", err, out)
	}

	for _, args := range [][]string{
		{"tag", "lightweight"},
		{"tag", "-a", "missing", "-m", "No authority"},
		{"tag", "-a", "duplicate", "-m", "Glade-Tools-SHA: " + testToolsSHA + "\nGlade-Tools-SHA: " + strings.Repeat("4", 40)},
		{"tag", "-a", "uppercase", "-m", "Glade-Tools-SHA: " + strings.Repeat("A", 40)},
		{"tag", "-a", "body", "-m", "Glade-Tools-SHA: " + testToolsSHA + "\n\nRelease notes continue."},
	} {
		runGitSF(t, root, args...)
		tag := args[1]
		if args[1] == "-a" {
			tag = args[2]
		}
		cmd := exec.Command(filepath.Join(root, "scripts", "verify-salesforce-check.sh"), "--tag-tools-sha", tag)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("invalid tag %s accepted: %s", tag, out)
		}
	}
}

func makeSalesforceCheckFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"scripts", ".github", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile("verify-salesforce-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(root, "scripts", "verify-salesforce-check.sh"), string(script))
	anchor, err := os.ReadFile(filepath.Join("..", ".github", "release-authorities.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "release-authorities.json"), anchor, 0o644); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(root, "bin", "gh"), "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" > \"$FAKE_GH_ARGS\"\nprintf '%s' \"$FAKE_GH_RESPONSE\"\n")
	return root
}

func runSalesforceCheckFixture(root, response string, args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(root, "scripts", "verify-salesforce-check.sh"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+":"+os.Getenv("PATH"), "GITHUB_REPOSITORY=glade-sh/glade", "FAKE_GH_ARGS="+filepath.Join(root, "gh.args"), "FAKE_GH_RESPONSE="+response)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runGitSF(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
