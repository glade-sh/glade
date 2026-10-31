package scripts

import (
	"fmt"
	"strings"
	"testing"
)

func TestCIRelease2PackageShardWorkflow(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	for _, pkg := range []string{"vm", "gladecli"} {
		job := jobs[pkg]
		for _, marker := range []string{"shard: [0, 1, 2, 3]", "scripts/ci-go-test.sh " + pkg + "-shard", pkg + "-history-input", "name: " + pkg + "-shard-${{ matrix.shard }}", "${{ github.run_attempt }}-${{ matrix.shard }}"} {
			if !strings.Contains(job, marker) {
				t.Errorf("%s shard job missing %q", pkg, marker)
			}
		}
		input := jobs[pkg+"-history-input"]
		for _, marker := range []string{"CI_DURATION_HISTORY_CACHE_REF", pkg + "-duration-history-v1", "name: " + pkg + "-history-input"} {
			if !strings.Contains(input, marker) {
				t.Errorf("%s input missing %q", pkg, marker)
			}
		}
		history := jobs[pkg+"-history"]
		for _, marker := range []string{"needs: " + pkg, "if: ${{ success() }}", "pattern: " + pkg + "-shard-*", pkg + "-duration-history-v1"} {
			if !strings.Contains(history, marker) {
				t.Errorf("%s history missing %q", pkg, marker)
			}
		}
		if !strings.Contains(history, "scripts/ci-go-test.sh "+pkg+"-history-refresh") {
			t.Errorf("%s history collector is missing", pkg)
		}
		for i := 0; i < 4; i++ {
			if !strings.Contains(history, fmt.Sprintf("%s-shard-%d", pkg, i)) {
				t.Errorf("%s history omits shard %d", pkg, i)
			}
		}
		for _, name := range []string{pkg, pkg + "-history"} {
			if !strings.Contains(jobs["required-ci"], "      - "+name+"\n") {
				t.Errorf("Required CI omits %s", name)
			}
		}
	}
	if !strings.Contains(jobs["vm-history"], `CI_VM_HISTORY_COMPLETE: "1"`) {
		t.Error("VM validated deferrals must be retained in history")
	}
}
