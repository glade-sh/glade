package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestCICase319RetryCacheSaveRestorePathMatches(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	jobs := workflowJobBlocks(t, string(data))
	restore := workflowStepBlock(t, jobs["apextest-history-input"], "      - name: Restore Apex retry scheduling profile")
	save := workflowStepBlock(t, jobs["apextest-history"], "      - name: Cache Apex scheduling profile after balance failure")
	for _, step := range []string{restore, save} {
		if !strings.Contains(step, "path: /tmp/apextest-scheduling-profile") {
			t.Errorf("retry cache save and restore must use the same path: %s", step)
		}
	}
	helper, err := os.ReadFile("ci-apex-history-input.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(helper), "CI_APEXTEST_RETRY_PROFILE_PATH:-/tmp/apextest-scheduling-profile/profile.json") {
		t.Error("helper does not read the restored cache path")
	}
}

func TestCICase319RetryCacheKeysUseBenchmarkPairPrefix(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	jobs := workflowJobBlocks(t, string(data))
	for _, tc := range []struct{ job, name, attempt string }{
		{"apextest-history-input", "Restore Apex retry scheduling profile", "${{ steps.retry.outputs.attempt }}"},
		{"apextest-history", "Cache Apex scheduling profile after balance failure", "${{ github.run_attempt }}"},
	} {
		t.Run(tc.job, func(t *testing.T) {
			step := workflowStepBlock(t, jobs[tc.job], "      - name: "+tc.name)
			want := "key: " + benchmarkCachePairPrefix + "apextest-scheduling-v1-${{ runner.os }}-${{ runner.arch }}-${{ github.run_id }}-" + tc.attempt
			if !strings.Contains(step, want) {
				t.Errorf("scheduling cache key must use the benchmark-pair prefix and matching attempt identity: want %q", want)
			}
		})
	}
}

func TestCICase319BenchmarkContractIgnoresUnrelatedCacheConditions(t *testing.T) {
	workflow := readWorkflowFile(t, "ci.yml")
	if problem := benchmarkCachePairWorkflowProblem("ci.yml", workflow); problem != "" {
		t.Fatalf("benchmark contract rejected independent scheduling-cache conditions: %s", problem)
	}
	condition := "        if: github.event_name != 'workflow_dispatch' || inputs.benchmark_cache_pair == 0"
	mutated := strings.Replace(workflow, condition, "        if: always()", 1)
	if mutated == workflow || benchmarkCachePairWorkflowProblem("ci.yml", mutated) == "" {
		t.Fatal("benchmark contract accepted a normal npm cache without its isolation condition")
	}
}
