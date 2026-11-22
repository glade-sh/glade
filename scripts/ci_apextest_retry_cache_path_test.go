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
	restore := workflowStepBlock(t, jobs["history-input"], "      - name: Restore Apex retry scheduling profile")
	save := workflowStepBlock(t, jobs["history"], "      - name: Cache Apex scheduling profile after balance failure")
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

func TestCICase319RetryCacheKeysUseAttemptIdentity(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	jobs := workflowJobBlocks(t, string(data))
	for _, tc := range []struct{ job, name, attempt string }{
		{"history-input", "Restore Apex retry scheduling profile", "${{ steps.retry.outputs.attempt }}"},
		{"history", "Cache Apex scheduling profile after balance failure", "${{ github.run_attempt }}"},
	} {
		t.Run(tc.job, func(t *testing.T) {
			step := workflowStepBlock(t, jobs[tc.job], "      - name: "+tc.name)
			want := "key: " + "apextest-scheduling-v1-${{ runner.os }}-${{ runner.arch }}-${{ github.run_id }}-" + tc.attempt
			if !strings.Contains(step, want) {
				t.Errorf("scheduling cache key must use the matching attempt identity: want %q", want)
			}
		})
	}
}
