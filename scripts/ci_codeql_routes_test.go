package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCICodeQLFastAndFullWorkflowRoutes(t *testing.T) {
	_, ci := readCIWorkflow(t)
	security := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))
	for _, route := range []struct{ name, job, profile string }{
		{"pull_request_any_size_and_main_push", ci["codeql"], "fast"},
		{"weekly_release_tag_and_manual", security["codeql"], "full"},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, marker := range []string{"uses: ./.github/workflows/codeql.yml", "profile: " + route.profile, "security-events: write"} {
				if !strings.Contains(route.job, marker) {
					t.Errorf("route omits %q", marker)
				}
			}
		})
	}
	if !strings.Contains(ci["required-ci"], "      - codeql\n") {
		t.Error("Required CI does not aggregate fast CodeQL")
	}
	if strings.Contains(ci["codeql"], "if:") {
		t.Error("fast scan must run for every CI event")
	}
	if !strings.Contains(security["codeql"], "if: github.event_name == 'schedule' || github.ref_type == 'tag' || github.event_name == 'workflow_dispatch'") {
		t.Error("full CodeQL must stay outside pull-request and main-push gates")
	}
	text := readWorkflowFile(t, "codeql.yml")
	job := workflowJobBlocks(t, text)["codeql"]
	for _, marker := range []string{"workflow_call:", "if: inputs.profile == 'fast'", "if: inputs.profile == 'full'", "timeout-minutes: 25", "python3 scripts/ci-codeql.py check", "CODEQL_ACTION_ANALYZE_DID_COMPLETE_SUCCESSFULLY=true", "if: steps.analyze.outputs.completed == 'true'"} {
		if !strings.Contains(text, marker) {
			t.Errorf("reusable workflow omits %q", marker)
		}
	}
	if strings.Contains(text, "changed_files") || strings.Contains(job, "github/codeql-action/analyze@") {
		t.Error("scan must use the selected profile and the bounded CLI wrapper")
	}
}

// Run the actual workflow command against an owned CLI stand-in, then feed its
// result into the real aggregate shell. Both profiles share this path.
func TestCICodeQLBoundedAnalysisAndMergeGateRoutes(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	aggregate := requiredAggregateShell(t, jobs["required-ci"])
	for _, profile := range []string{"fast", "full"} {
		for _, tc := range []struct {
			name string
			fail bool
		}{
			{"clean", false}, {"alert", true}, {"suppressed", false}, {"note_alert", true},
			{"analysis_error", true}, {"exit_124_is_error", true}, {"malformed_sarif", true},
			{"query_timeout", true}, {"finalize_timeout", true}, {"interpret_timeout", true},
		} {
			t.Run(profile+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				cli := filepath.Join(dir, "codeql")
				fake := `#!/usr/bin/env python3
import json, os, sys, time
mode = os.environ['CODEQL_FIXTURE']
phase = sys.argv[2]
if mode == {'finalize':'finalize_timeout','run-queries':'query_timeout','interpret-results':'interpret_timeout'}[phase]:
    if phase == 'interpret-results':
        output = next(a.split('=', 1)[1] for a in sys.argv if a.startswith('--output='))
        with open(output, 'w') as f: f.write(json.dumps({'version':'2.1.0','runs':[{'tool':{'driver':{'name':'CodeQL'}},'results':[]}]}))
    time.sleep(30)
if phase == 'run-queries' and mode == 'analysis_error': sys.exit(2)
if phase == 'run-queries' and mode == 'exit_124_is_error': sys.exit(124)
if phase == 'interpret-results':
    output = next(a.split('=', 1)[1] for a in sys.argv if a.startswith('--output='))
    results = [] if mode == 'clean' else [{'ruleId':'go/synthetic', 'level':'note' if mode == 'note_alert' else 'warning', 'message':{'text':'owned fixture'}}]
    if mode == 'suppressed': results[0]['suppressions'] = [{'status':'accepted','kind':'inSource'}]
    with open(output, 'w') as f: f.write('invalid' if mode == 'malformed_sarif' else json.dumps({'version':'2.1.0','runs':[{'tool':{'driver':{'name':'CodeQL'}},'results':results}]}))
`
				if err := os.WriteFile(cli, []byte(fake), 0700); err != nil {
					t.Fatal(err)
				}
				budget := filepath.Join(dir, "budget.json")
				budgetJSON := `{"analyze_seconds":15,"query_seconds":10}`
				if strings.HasSuffix(tc.name, "_timeout") {
					budgetJSON = `{"analyze_seconds":5,"query_seconds":3}`
				}
				if err := os.WriteFile(budget, []byte(budgetJSON), 0600); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(dir, "result.sarif")
				githubOutput := filepath.Join(dir, "output")
				githubEnv := filepath.Join(dir, "env")
				run := exec.Command("python3", "ci-codeql.py", "analyze", "--profile", profile, "--codeql", cli, "--database", dir, "--output", output, "--budget", budget)
				run.Env = append(os.Environ(), "CODEQL_FIXTURE="+tc.name, "GITHUB_OUTPUT="+githubOutput, "GITHUB_ENV="+githubEnv, "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
				out, err := run.CombinedOutput()
				if strings.Contains(string(out), "can't open file") {
					t.Fatalf("workflow command missing: %s", out)
				}
				timedOut := strings.HasSuffix(tc.name, "_timeout")
				phase := map[string]string{"query_timeout": "run-queries", "finalize_timeout": "finalize", "interpret_timeout": "interpret-results"}[tc.name]
				if timedOut && (err == nil || !strings.Contains(string(out), "CodeQL "+phase) || !strings.Contains(string(out), "analysis is incomplete")) {
					t.Fatalf("timeout must fail with a clear message: %v\n%s", err, out)
				}
				data, _ := os.ReadFile(githubOutput)
				if timedOut && (!strings.Contains(string(data), "completed=false") || strings.Contains(string(data), "completed=true")) {
					t.Fatalf("timeout must not upload stale results: %s", data)
				}
				if timedOut {
					if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
						t.Fatalf("timeout left partial SARIF: %v", statErr)
					}
					env, _ := os.ReadFile(githubEnv)
					for _, flag := range []string{"CODEQL_ACTION_SKIP_SARIF_UPLOAD=true", "CODEQL_ACTION_JOB_STATUS=JOB_STATUS_FAILURE"} {
						if !strings.Contains(string(env), flag) {
							t.Errorf("timeout omits pinned post-hook flag %s: %s", flag, env)
						}
					}
				}
				if err == nil && !timedOut {
					if !strings.Contains(string(data), "completed=true") {
						t.Fatalf("successful analysis must allow upload: %s\n%s", data, out)
					}
					out, err = exec.Command("python3", "ci-codeql.py", "check", output).CombinedOutput()
				}
				if (err != nil) != tc.fail {
					t.Fatalf("failure=%v, want %v: %s", err, tc.fail, out)
				}
				result := "success"
				if err != nil {
					result = "failure"
				}
				needs, _ := json.Marshal(map[string]any{"codeql": map[string]string{"result": result}, "test": map[string]string{"result": "success"}})
				gate := exec.Command("bash", "-c", aggregate)
				gate.Env = append(os.Environ(), "REQUIRED_RESULTS="+string(needs))
				if out, err := gate.CombinedOutput(); (err != nil) != tc.fail {
					t.Fatalf("merge gate failure=%v, want %v: %s", err, tc.fail, out)
				}
			})
		}
	}
}

func TestCICodeQLDiffInformedQueryRoutes(t *testing.T) {
	for _, route := range []struct {
		name, profile, ranges string
		pack                  bool
	}{
		{"fast_small_pr", "fast", `[{"path":"src/quoted'name.go","startLine":2,"endLine":4}]`, true},
		{"fast_empty_diff", "fast", `[]`, true},
		{"fast_large_pr_or_missing_ranges", "fast", "", false},
		{"full_ignores_pr_ranges", "full", `[]`, false},
	} {
		t.Run(route.name, func(t *testing.T) {
			dir := t.TempDir()
			if route.ranges != "" {
				if err := os.WriteFile(filepath.Join(dir, "pr-diff-range.json"), []byte(route.ranges), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cli := filepath.Join(dir, "codeql")
			fake := `#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
phase = sys.argv[2]
Path(os.environ['CODEQL_CALL_DIR'], phase + '.json').write_text(json.dumps(sys.argv))
if phase == 'interpret-results':
    output = next(a.split('=',1)[1] for a in sys.argv if a.startswith('--output='))
    Path(output).write_text(json.dumps({'version':'2.1.0','runs':[{'tool':{'driver':{'name':'CodeQL'}},'results':[]}]}))
`
			if err := os.WriteFile(cli, []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "ci-codeql.py", "analyze", "--profile", route.profile, "--codeql", cli, "--database", dir, "--output", filepath.Join(dir, "result.sarif"), "--budget", "../.github/codeql-budget.json")
			cmd.Env = append(os.Environ(), "CODEQL_ACTION_TEMP="+dir, "GITHUB_WORKSPACE="+dir, "CODEQL_CALL_DIR="+dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("diff route: %v\n%s", err, out)
			}
			body, err := os.ReadFile(filepath.Join(dir, "run-queries.json"))
			if err != nil {
				t.Fatal(err)
			}
			// The evaluator's default Java heap ran out on hosted runners; pin
			// an explicit RAM budget below the 16 GB runner limit.
			var queryArgs []string
			if err := json.Unmarshal(body, &queryArgs); err != nil {
				t.Fatal(err)
			}
			ram := 0
			for _, arg := range queryArgs {
				if strings.HasPrefix(arg, "--ram=") || arg == "--ram" || arg == "-M" {
					ram++
				}
			}
			if ram != 1 || !strings.Contains(string(body), `"--ram=8192"`) || !strings.Contains(string(body), `"--threads=0"`) {
				t.Fatalf("run-queries must pass --threads=0 and exactly one --ram=8192: %s", body)
			}
			for _, phase := range []string{"finalize", "interpret-results"} {
				other, err := os.ReadFile(filepath.Join(dir, phase+".json"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(other), "--ram") {
					t.Fatalf("%s must keep its existing options: %s", phase, other)
				}
			}
			interpret, _ := os.ReadFile(filepath.Join(dir, "interpret-results.json"))
			if strings.Contains(string(interpret), "--sarif-run-property=incrementalMode=diff-informed") != route.pack {
				t.Fatalf("diff SARIF marker: %s", interpret)
			}
			if strings.Contains(string(body), "--extension-packs=codeql-action/pr-diff-range") != route.pack {
				t.Fatalf("diff extension presence: %s", body)
			}
			if route.pack {
				var args []string
				if err := json.Unmarshal(body, &args); err != nil {
					t.Fatal(err)
				}
				var pack string
				for _, arg := range args {
					if strings.HasPrefix(arg, "--additional-packs=") {
						pack = strings.TrimPrefix(arg, "--additional-packs=")
					}
				}
				contents, err := os.ReadFile(filepath.Join(pack, "diff-ranges.yml"))
				if err != nil {
					t.Fatal(err)
				}
				var extension struct{ Extensions []struct{ Data [][]any } }
				if err := json.Unmarshal(contents, &extension); err != nil || len(extension.Extensions) != 1 || len(extension.Extensions[0].Data) != 1 {
					t.Fatalf("bad extension: %v %s", err, contents)
				}
				row := extension.Extensions[0].Data[0]
				if route.ranges == "[]" {
					if row[0] != "" {
						t.Fatal("empty diff must restrict all alerts")
					}
				} else if row[0] != filepath.ToSlash(filepath.Join(dir, "src/quoted'name.go")) || row[1] != float64(2) || row[2] != float64(4) {
					t.Fatalf("diff row changed: %v", row)
				}
			}
		})
	}
}
