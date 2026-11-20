package scripts

import (
	"encoding/json"
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
		{"squash merge", tree, "success", merge, head, true},
		{"different tree", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "success", merge, head, false},
		{"failed run", tree, "failure", merge, head, false},
		{"missing receipt", tree, "success", "", head, false},
		{"wrong merge parent", tree, "success", merge, base, false},
		{"receipt for another run", tree, "success", merge, head, false},
		{"fast-only pull request", tree, "success", merge, head, false},
		{"missing coverage", tree, "success", merge, head, false},
		{"failed coverage", tree, "success", merge, head, false},
		{"skipped required job", tree, "success", merge, head, false},
		{"missing job", tree, "success", merge, head, false},
		{"changed workflow", tree, "success", merge, head, false},
		{"wrong attempt", tree, "success", merge, head, false},
		{"wrong push parent", tree, "success", merge, head, false},
		{"incomplete run", tree, "success", merge, head, false},
		{"missing receipt job", tree, "success", merge, head, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			gh := `#!/usr/bin/env bash
case "$*" in
 *"/commits/` + push + `/pulls"*) printf '[{"number":7,"merged_at":"2026-01-01","merge_commit_sha":"` + push + `","base":{"ref":"main","repo":{"full_name":"glade-sh/glade"}},"head":{"sha":"` + head + `","ref":"%s","repo":{"full_name":"glade-sh/glade"}}}]' "$PR_REF" ;;
 *"/contents/.github/workflows/ci.yml"*) if [[ "$*" == *".content"* ]]; then base64 < ../.github/workflows/ci.yml; elif [[ "$SCENARIO" == 'changed workflow' && "$*" == *"ref=` + push + `"* ]]; then printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'; else printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'; fi ;;
 *"/actions/workflows/ci.yml/runs"*) printf '{"workflow_runs":[{"id":101,"run_attempt":1,"head_sha":"` + head + `","head_branch":"%s","event":"pull_request","status":"%s","conclusion":"%s","pull_requests":[{"number":7}]}]}' "$PR_REF" "${RUN_STATUS:-completed}" "$CONCLUSION" ;;
 *"/actions/runs/101/attempts/1/jobs"*) if [[ "$SCENARIO" == "missing receipt job" ]]; then printf '[{"jobs":[{"name":"Required CI","status":"completed","conclusion":"success"}]}]'; else printf '[{"jobs":[{"name":"Required CI","status":"completed","conclusion":"success"},{"name":"Tree receipt","status":"completed","conclusion":"success"}]}]'; fi ;;
 *"/commits/` + push + `"*) if [[ "$SCENARIO" == 'squash merge' ]]; then printf '{"sha":"` + push + `","commit":{"tree":{"sha":"%s"}},"parents":[{"sha":"` + base + `"}]}' "$PUSH_TREE"; else printf '{"sha":"` + push + `","commit":{"tree":{"sha":"%s"}},"parents":[{"sha":"` + base + `"},{"sha":"%s"}]}' "$PUSH_TREE" "${PUSH_HEAD:-` + head + `}"; fi ;;
 *"/commits/` + merge + `"*) printf '{"sha":"` + merge + `","commit":{"tree":{"sha":"` + tree + `"}},"parents":[{"sha":"` + base + `"},{"sha":"%s"}]}' "$PARENT" ;;
 *"run download 101"*)
   out=""; while [[ $# -gt 0 ]]; do [[ "$1" == "-D" ]] && out="$2"; shift; done
   [[ -n "$RECEIPT" ]] || exit 1
   awk '/^  required-ci:$/ {job=1; next} job && /^  [a-zA-Z0-9_-]+:$/ {exit} job && /^    needs:$/ {needs=1; next} needs && /^      - [a-zA-Z0-9_-]+$/ {print $2; next} needs {exit}' ../.github/workflows/ci.yml | jq -Rsc 'split("\n") | map(select(length > 0)) | map({key: ., value: {result: (if . == "pr-tests" or . == "pr-guards" then "skipped" else "success" end)}}) | from_entries' > "$out/coverage.json"
   case "$SCENARIO" in
     'missing job') jq 'del(.site)' "$out/coverage.json" > "$out/c.tmp" && mv "$out/c.tmp" "$out/coverage.json" ;;
     'skipped required job') jq '.site.result = "skipped"' "$out/coverage.json" > "$out/c.tmp" && mv "$out/c.tmp" "$out/coverage.json" ;;
     'failed coverage') jq '.site.result = "failure"' "$out/coverage.json" > "$out/c.tmp" && mv "$out/c.tmp" "$out/coverage.json" ;;
   esac
   jq -n --arg checkout "$RECEIPT" --argjson run "${RECEIPT_RUN:-101}" --argjson coverage "$(cat "$out/coverage.json")" --argjson attempt "$RECEIPT_ATTEMPT" '{checkout_sha:$checkout, run_id:$run, run_attempt:$attempt, pr_number:7, coverage:$coverage}' > "$out/ci-tree.json"
   if [[ "$SCENARIO" == 'missing coverage' ]]; then jq 'del(.coverage)' "$out/ci-tree.json" > "$out/c.tmp" && mv "$out/c.tmp" "$out/ci-tree.json"; fi ;;
 *) echo "unexpected gh call: $*" >&2; exit 1 ;;
esac`
			if err := os.WriteFile(filepath.Join(d, "gh"), []byte(gh), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-verify-pr-tree.sh", "glade-sh/glade", push)
			cmd.Env = append(os.Environ(), "PATH="+d+":"+os.Getenv("PATH"), "PUSH_TREE="+tc.pushTree, "CONCLUSION="+tc.conclusion, "RECEIPT="+tc.receipt, "RECEIPT_RUN="+map[bool]string{true: "999", false: "101"}[tc.name == "receipt for another run"], "PARENT="+tc.parent, "PR_REF="+map[bool]string{true: "feature/example", false: "train/example"}[tc.name == "fast-only pull request"], "SCENARIO="+tc.name, "PUSH_HEAD="+map[bool]string{true: base, false: head}[tc.name == "wrong push parent"], "RUN_STATUS="+map[bool]string{true: "in_progress", false: "completed"}[tc.name == "incomplete run"], "RECEIPT_ATTEMPT="+map[bool]string{true: "2", false: "1"}[tc.name == "wrong attempt"])
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected result %v: %s", err, out)
			}
			if tc.ok && !strings.Contains(string(out), tree) {
				t.Fatalf("missing tree evidence: %s", out)
			}
			if tc.ok {
				var receipt struct {
					RunAttempt int `json:"run_attempt"`
				}
				if err := json.Unmarshal(out, &receipt); err != nil || receipt.RunAttempt != 1 {
					t.Fatalf("missing verified run attempt: %v: %s", err, out)
				}
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
