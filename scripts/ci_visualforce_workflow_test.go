package scripts

import (
	"fmt"
	"strings"
	"testing"
)

func TestCIVisualforceWorkflowSharesInputAndRequiresUnion(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	input, matrix, refresh := jobs["visualforce-history-input"], jobs["test"], jobs["visualforce-history"]
	for _, marker := range []string{
		"actions: read", `-f head_sha="$CI_DURATION_HISTORY_CACHE_REF" -f status=success`,
		"select(.expired == false)", `for index in {0..3}`, "format=shards", "format=serial",
		"if: steps.predecessor.outputs.format == 'shards'", "if: steps.predecessor.outputs.format == 'serial'",
		"pattern: go-test-visualforce-shard-*", "name: go-test-remaining-go",
		"run-id: ${{ steps.predecessor.outputs.run_id }}", "github-token: ${{ github.token }}",
		"visualforce-history-bootstrap", "/tmp/visualforce-predecessor/test-remaining-go-visualforce.json",
		"visualforce-history-refresh", "go-test-visualforce-shard-{0..3}",
		`rm -f "$CI_VISUALFORCE_HISTORY_PATH"`, "using deterministic scheduling fallback",
		"name: visualforce-history-input", "CI_VISUALFORCE_HISTORY_PATH",
	} {
		if !strings.Contains(input, marker) {
			t.Errorf("Visualforce input missing %q", marker)
		}
	}
	if strings.Contains(input, "actions/cache/save@") {
		t.Error("history producer must not publish unvalidated run results")
	}
	if strings.Count(input, "if: ${{ github.event_name != 'workflow_dispatch' || inputs.benchmark_cache_pair == 0 }}") != 1 {
		t.Error("ordinary predecessor evidence must not seed a positive benchmark pair")
	}
	for index := range 4 {
		row := fmt.Sprintf("          - lane: visualforce-%d\n            name: Visualforce (%d)\n            shard: %d\n", index, index, index)
		if strings.Count(matrix, row) != 1 {
			t.Errorf("Visualforce shard %d must occur exactly once", index)
		}
	}
	for _, step := range []string{"Download shared Visualforce duration history", "Test Visualforce shard", "Upload Visualforce shard events"} {
		block := workflowStepBlockText(matrix, "      - name: "+step)
		if !strings.Contains(block, "startsWith(matrix.lane, 'visualforce-')") {
			t.Errorf("%s must only apply to Visualforce workers", step)
		}
	}
	for _, marker := range []string{
		"needs: [lwc-compile-history-input, visualforce-history-input, test-binaries]",
		"CI_VISUALFORCE_HISTORY_PATH: /tmp/glade-visualforce-duration-history/visualforce-duration-history.json",
		"name: visualforce-history-input", "path: /tmp/glade-visualforce-duration-history",
		`scripts/ci-go-test.sh visualforce-shard "${{ matrix.shard }}"`,
		"name: go-test-visualforce-shard-${{ matrix.shard }}", "path: ci-artifacts/visualforce-${{ matrix.shard }}/",
	} {
		if !strings.Contains(matrix, marker) {
			t.Errorf("Visualforce worker missing %q", marker)
		}
	}
	if strings.Contains(matrix, "visualforce-duration-history-v1") || strings.Contains(matrix, "gh api") {
		t.Error("workers must consume the same immutable history input")
	}
	for _, marker := range []string{
		"needs: test", "if: ${{ success() }}", "pattern: go-test-visualforce-shard-*",
		"scripts/ci-go-test.sh visualforce-history-refresh", "go-test-visualforce-shard-{0..3}",
		"actions/cache/save@", "visualforce-duration-history-v1", "github.sha",
	} {
		if !strings.Contains(refresh, marker) {
			t.Errorf("Visualforce coverage/history guard missing %q", marker)
		}
	}
	if !strings.Contains(jobs["required-ci"], "      - visualforce-history\n") {
		t.Error("required CI must require the complete Visualforce union")
	}
}
