package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the release command itself; this test also compiles before the
// command exists, so its first failure identifies the missing budget gate.
func TestCodeQLBudgetReleaseCommandRoutes(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "codeql-budget")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build budget command: %v\n%s", err, out)
	}
	workflow := `jobs:
  codeql:
    steps:
      - name: Initialize pull request CodeQL
        with:
          config: |
            query-filters:
              - exclude:
                  id: [go/first]
      - name: Initialize full-branch CodeQL
        with:
          config: |
            query-filters:
              - exclude:
                  id: [go/first, go/second, go/first]
`
	budget := `{"full_waivers":2,"pr_waivers":1,"analyze_seconds":300,"query_seconds":240}`
	report := `{"jobs":[{"name":"CodeQL","conclusion":"success","steps":[
 {"name":"Initialize full-branch CodeQL","conclusion":"success"},
 {"name":"Initialize pull request CodeQL","conclusion":"skipped"},
 {"name":"Analyze CodeQL","conclusion":"success","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:04:00Z"}]}]}`
	log := "CodeQL\tUNKNOWN STEP\t2026-01-01T00:00:01Z [command]/bin/codeql database run-queries --threads=4\n" +
		"CodeQL\tUNKNOWN STEP\t2026-01-01T00:03:01Z [1/2 eval 1m] Evaluation done; writing results to one.bqrs.\n" +
		"CodeQL\tUNKNOWN STEP\t2026-01-01T00:03:31Z [2/2 eval 2m] Evaluation done; writing results to two.bqrs.\n"
	reusableReport := strings.ReplaceAll(report, `"name":"CodeQL"`, `"name":"CodeQL / CodeQL (full)"`)
	reusableReport = strings.Replace(reusableReport, `"steps":[`, `"steps":[{"name":"Upload completed CodeQL results","conclusion":"success"},{"name":"Fail on CodeQL alerts","conclusion":"success"},`, 1)
	cases := []struct {
		name, workflow, budget, report, log, want string
		fail                                      bool
	}{
		{"full_weekly_release_tag_dispatch", workflow, budget, report, log, "full waivers=2/2; PR waivers=1/1; analyze=240.00/300s; queries=210.00/240s", false},
		{"reusable_full_job_name", workflow, budget, reusableReport, log, "analyze=240.00/300s", false},
		{"reusable_timeout_after_queries", workflow, budget, strings.ReplaceAll(reusableReport, `"Upload completed CodeQL results","conclusion":"success"`, `"Upload completed CodeQL results","conclusion":"skipped"`), log, "full-profile result upload and alert check did not succeed", true},
		{"reusable_alert_check_missing", workflow, budget, strings.ReplaceAll(reusableReport, `"Fail on CodeQL alerts","conclusion":"success"`, `"Fail on CodeQL alerts","conclusion":"skipped"`), log, "full-profile result upload and alert check did not succeed", true},
		{"reusable_fast_cannot_prove_full", workflow, budget, strings.ReplaceAll(report, `"name":"CodeQL"`, `"name":"CodeQL / CodeQL (fast)"`), log, "expected one CodeQL job", true},
		{"full_time_boundary", workflow, budget, strings.ReplaceAll(report, "00:04:00Z", "00:05:00Z"), strings.ReplaceAll(log, "00:03:31Z", "00:04:01Z"), "queries=240.00/240s", false},
		{"full_waiver_growth", strings.ReplaceAll(workflow, "go/second, go/first", "go/second, go/third"), budget, report, log, "full waivers 3 exceed budget 2", true},
		{"pr_waiver_growth", strings.ReplaceAll(workflow, "id: [go/first]", "id: [go/first, go/third]"), budget, report, log, "PR waivers 2 exceed budget 1", true},
		{"full_analyze_over_budget", workflow, budget, strings.ReplaceAll(report, "00:04:00Z", "00:05:01Z"), log, "analyze time 301.00s exceeds budget 300s", true},
		{"full_queries_over_budget", workflow, budget, report, strings.ReplaceAll(log, "00:03:31Z", "00:04:02Z"), "query time 241.00s exceeds budget 240s", true},
		{"small_pr_cannot_prove_full_profile", workflow, budget, strings.ReplaceAll(report, `"Initialize full-branch CodeQL","conclusion":"success"`, `"Initialize full-branch CodeQL","conclusion":"skipped"`), log, "full-profile initialization did not succeed", true},
		{"failed_job", workflow, budget, strings.Replace(report, `"conclusion":"success"`, `"conclusion":"failure"`, 1), log, "CodeQL job did not succeed", true},
		{"missing_analysis", workflow, budget, strings.ReplaceAll(report, "Analyze CodeQL", "Other step"), log, "expected one successful Analyze CodeQL step", true},
		{"invalid_timestamp", workflow, budget, strings.ReplaceAll(report, "2026-01-01T00:04:00Z", "invalid"), log, "invalid Analyze CodeQL timestamps", true},
		{"negative_duration", workflow, budget, strings.ReplaceAll(report, "2026-01-01T00:04:00Z", "2025-01-01T00:04:00Z"), log, "invalid Analyze CodeQL timestamps", true},
		{"incomplete_query_log", workflow, budget, report, strings.ReplaceAll(log, "[2/2 eval", "[1/2 eval"), "incomplete query evaluation", true},
		{"query_log_outside_analyze", workflow, budget, report, strings.ReplaceAll(log, "2026-01-01T00:00:01Z", "2025-12-31T23:59:59Z"), "query log timestamps fall outside Analyze CodeQL", true},
		{"duplicate_query_launch", workflow, budget, report, log + strings.Split(log, "\n")[0] + "\n", "expected one query evaluation", true},
		{"conflicting_initialization_routes", workflow, budget, strings.ReplaceAll(report, `"Initialize pull request CodeQL","conclusion":"skipped"`, `"Initialize pull request CodeQL","conclusion":"success"`), log, "small-PR initialization cannot prove the full profile", true},
		{"empty_query_log", workflow, budget, report, "", "expected one query evaluation", true},
		{"missing_profile", strings.ReplaceAll(workflow, "Initialize full-branch CodeQL", "Other step"), budget, report, log, "expected one Initialize full-branch CodeQL config", true},
		{"unsupported_waiver_filter", strings.ReplaceAll(workflow, "id: [go/first]", "tags: [security]"), budget, report, log, "exclusions must use explicit query IDs", true},
		{"invalid_budget", workflow, strings.ReplaceAll(budget, `"query_seconds":240`, `"query_seconds":0`), report, log, "budgets must be positive", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{"workflow.yml": tc.workflow, "budget.json": tc.budget, "run.json": tc.report, "query.log": tc.log}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workflow", filepath.Join(dir, "workflow.yml"), "-budget", filepath.Join(dir, "budget.json"), "-run-json", filepath.Join(dir, "run.json"), "-query-log", filepath.Join(dir, "query.log")}
			out, err := exec.Command(binary, args...).CombinedOutput()
			if (err != nil) != tc.fail || !strings.Contains(string(out), tc.want) {
				t.Fatalf("exit=%v; want failure=%v, output containing %q; got %s", err, tc.fail, tc.want, out)
			}
		})
	}
	t.Run("configuration_only", func(t *testing.T) {
		out, err := exec.Command(binary, "-workflow", "../../.github/workflows/codeql.yml", "-budget", "../../.github/codeql-budget.json").CombinedOutput()
		if err != nil || !strings.Contains(string(out), "full waivers=12/12; PR waivers=7/7") {
			t.Fatalf("recorded budget: %v\n%s", err, out)
		}
		var recorded map[string]int
		body, err := os.ReadFile("../../.github/codeql-budget.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &recorded); err != nil {
			t.Fatal(err)
		}
		if recorded["analyze_seconds"] != 300 || recorded["query_seconds"] != 240 {
			t.Fatalf("unexpected release budgets: %v", recorded)
		}
	})
}
