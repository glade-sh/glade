package scripts

import (
	"fmt"
	"strings"
	"testing"
)

func TestCIRelease2PackageShardWorkflow(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	// Push CI runs three VM and two CLI shards to stay inside the job budget;
	// PR workers keep the four-shard layout and select it by leaving the count unset.
	counts := map[string]int{"vm": 3, "gladecli": 2}
	for _, pkg := range []string{"vm", "gladecli"} {
		job := jobs[pkg]
		matrix := "shard: [0, 1, 2]"
		if counts[pkg] == 2 {
			matrix = "shard: [0, 1]"
		}
		countEnv := fmt.Sprintf("CI_%s_SHARDS: \"%d\"", strings.ToUpper(pkg), counts[pkg])
		for _, marker := range []string{matrix, countEnv, "scripts/ci-go-test.sh " + pkg + "-shard", pkg + "-history-input", "name: " + pkg + "-shard-${{ matrix.shard }}", "${{ github.run_attempt }}-${{ matrix.shard }}"} {
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
		for i := 0; i < counts[pkg]; i++ {
			if !strings.Contains(history, fmt.Sprintf("%s-shard-%d", pkg, i)) {
				t.Errorf("%s history omits shard %d", pkg, i)
			}
		}
		if strings.Contains(history, fmt.Sprintf("%s-shard-%d", pkg, counts[pkg])) {
			t.Errorf("%s history names a shard beyond the matrix", pkg)
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
