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

func TestReleasePreflightChecksBothAuthorities(t *testing.T) {
	for _, missing := range []string{"", "ci", "salesforce"} {
		t.Run("missing-"+missing, func(t *testing.T) {
			root := makeSalesforceCheckFixture(t)
			for _, name := range []string{"release-preflight.sh", "verify-required-ci.sh"} {
				writeExecutable(t, filepath.Join(root, "scripts", name), string(mustReadFile(t, name)))
			}
			writeExecutable(t, filepath.Join(root, "bin", "gh"), `#!/usr/bin/env bash
set -eu
case "$*" in
  *actions/workflows/ci.yml/runs*)
    if [[ "$MISSING" == ci ]]; then printf '{"workflow_runs":[]}'; else
      printf '{"workflow_runs":[{"id":101,"head_sha":"%s","conclusion":"success","event":"push","created_at":"2026-09-04","html_url":"https://github.com/glade-sh/glade/actions/runs/101"}]}' "$GLADE_SHA"
    fi ;;
  *actions/runs/101/jobs*) printf '{"jobs":[{"id":501,"name":"Required CI","head_sha":"%s","status":"completed","conclusion":"success","html_url":"https://github.com/glade-sh/glade/actions/runs/101/job/501"}]}' "$GLADE_SHA" ;;
  *check-runs*)
    if [[ "$MISSING" == salesforce ]]; then printf '{"total_count":0,"check_runs":[]}'; else printf '%s' "$SF_RESPONSE"; fi ;;
  *) exit 97 ;;
esac
`)
			response := fmt.Sprintf(`{"total_count":1,"check_runs":[{"id":42,"name":"Salesforce Correctness","head_sha":"%s","status":"completed","conclusion":"success","external_id":"salesforce-release-authority/v1;tools_sha=%s;run_id=123;run_attempt=2;receipt_sha256=%s","html_url":"https://github.com/glade-sh/glade/runs/42","app":{"id":4101915}}]}`, testGladeSHA, testToolsSHA, testDigest)
			cmd := exec.Command("bash", filepath.Join(root, "scripts", "release-preflight.sh"), testGladeSHA, testToolsSHA)
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+":"+os.Getenv("PATH"), "GITHUB_REPOSITORY=glade-sh/glade", "GLADE_SHA="+testGladeSHA, "MISSING="+missing, "SF_RESPONSE="+response)
			out, err := cmd.CombinedOutput()
			if missing != "" {
				if err == nil || !strings.Contains(string(out), "Do not tag") {
					t.Fatalf("missing %s approval was not actionable: err=%v\n%s", missing, err, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("preflight: %v\n%s", err, out)
			}
			var receipt map[string]any
			if err := json.Unmarshal(out, &receipt); err != nil || receipt["gladeSHA"] != testGladeSHA || receipt["toolsSHA"] != testToolsSHA || receipt["requiredCI"] == nil || receipt["salesforce"] == nil {
				t.Fatalf("preflight did not bind both approvals: %v\n%s", err, out)
			}
		})
	}
}

func TestDistributionRunbookRequiresApprovedAnnotatedTag(t *testing.T) {
	doc := strings.Join(strings.Fields(string(mustReadFile(t, filepath.Join("..", "docs", "DISTRIBUTION_WORKFLOW.md")))), " ")
	for _, want := range []string{"scripts/release-preflight.sh", `git tag -a "$VERSION" "$GLADE_SHA"`, "Glade-Tools-SHA:", "Do not move", "default and pinned"} {
		if !strings.Contains(doc, want) {
			t.Errorf("distribution runbook missing %q", want)
		}
	}
	if strings.Contains(doc, "git tag vX.Y.Z") {
		t.Error("runbook still creates a lightweight tag")
	}
}
