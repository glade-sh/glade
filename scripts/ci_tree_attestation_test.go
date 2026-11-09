package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCITreeAttestation(t *testing.T) {
	const push = "1111111111111111111111111111111111111111"
	const head = "2222222222222222222222222222222222222222"
	const merge = "3333333333333333333333333333333333333333"
	const base = "4444444444444444444444444444444444444444"
	const tree = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cases := []struct {
		name, pushTree, conclusion, receipt, parent string
		ok                                          bool
	}{
		{"same tree", tree, "success", merge, head, true},
		{"different tree", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "success", merge, head, false},
		{"failed run", tree, "failure", merge, head, false},
		{"missing receipt", tree, "success", "", head, false},
		{"wrong merge parent", tree, "success", merge, base, false},
		{"receipt for another run", tree, "success", merge, head, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			gh := `#!/usr/bin/env bash
case "$*" in
 *"/commits/` + push + `/pulls"*) printf '[{"number":7,"merged_at":"2026-01-01","merge_commit_sha":"` + push + `","base":{"ref":"main"},"head":{"sha":"` + head + `"}}]' ;;
 *"/actions/workflows/ci.yml/runs"*) printf '{"workflow_runs":[{"id":101,"run_attempt":1,"head_sha":"` + head + `","event":"pull_request","status":"completed","conclusion":"%s","pull_requests":[{"number":7}]}]}' "$CONCLUSION" ;;
 *"/actions/runs/101/attempts/1/jobs"*) printf '[{"jobs":[{"name":"Required CI","status":"completed","conclusion":"success"}]}]' ;;
 *"/commits/` + push + `"*) printf '{"sha":"` + push + `","commit":{"tree":{"sha":"%s"}}}' "$PUSH_TREE" ;;
 *"/commits/` + merge + `"*) printf '{"sha":"` + merge + `","commit":{"tree":{"sha":"` + tree + `"}},"parents":[{"sha":"` + base + `"},{"sha":"%s"}]}' "$PARENT" ;;
 *"run download 101"*) out=""; while [[ $# -gt 0 ]]; do [[ "$1" == "-D" ]] && out="$2"; shift; done; if [[ -n "$RECEIPT" ]]; then printf '{"checkout_sha":"%s","run_id":%s,"run_attempt":1,"pr_number":7}' "$RECEIPT" "${RECEIPT_RUN:-101}" > "$out/ci-tree.json"; else exit 1; fi ;;
 *) echo "unexpected gh call: $*" >&2; exit 1 ;;
esac`
			if err := os.WriteFile(filepath.Join(d, "gh"), []byte(gh), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-verify-pr-tree.sh", "glade-sh/glade", push)
			cmd.Env = append(os.Environ(), "PATH="+d+":"+os.Getenv("PATH"), "PUSH_TREE="+tc.pushTree, "CONCLUSION="+tc.conclusion, "RECEIPT="+tc.receipt, "RECEIPT_RUN="+map[bool]string{true: "999", false: "101"}[tc.name == "receipt for another run"], "PARENT="+tc.parent)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected result %v: %s", err, out)
			}
			if tc.ok && !strings.Contains(string(out), tree) {
				t.Fatalf("missing tree evidence: %s", out)
			}
		})
	}
}

func TestVerifyRequiredCITreeFallbackIsOptInAndFailsClosed(t *testing.T) {
	d := t.TempDir()
	gh := `#!/usr/bin/env bash
case "$*" in
 *"/actions/workflows/ci.yml/runs"*) printf '{"workflow_runs":[]}' ;;
 *"/pulls"*) printf '[]' ;;
 *) echo "unexpected gh call: $*" >&2; exit 97 ;;
esac`
	if err := os.WriteFile(filepath.Join(d, "gh"), []byte(gh), 0700); err != nil {
		t.Fatal(err)
	}
	sha := "1111111111111111111111111111111111111111"
	for _, optIn := range []bool{false, true} {
		cmd := exec.Command("bash", "verify-required-ci.sh", "glade-sh/glade", sha)
		cmd.Env = append(os.Environ(), "PATH="+d+":"+os.Getenv("PATH"))
		if optIn {
			cmd.Env = append(cmd.Env, "REQUIRED_CI_ALLOW_TREE_ATTESTATION=1")
		}
		out, err := cmd.CombinedOutput()
		want := "no successful Required CI authority"
		if optIn {
			want = "tree attestation refused"
		}
		if err == nil || !strings.Contains(string(out), want) {
			t.Errorf("optIn=%v: err=%v out=%s", optIn, err, out)
		}
	}
}

func TestCIWorkflowWritesTreeReceiptOnPullRequests(t *testing.T) {
	_, ci := readCIWorkflow(t)
	block := ci["tree-receipt"]
	for _, marker := range []string{"name: Write tree receipt", "name: ci-tree", "github.event_name == 'pull_request'"} {
		if !strings.Contains(block, marker) {
			t.Errorf("tree-receipt missing %s", marker)
		}
	}
}
