package scripts

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The push layout runs two gladecli shards, so the history refresh must accept
// OUTPUT plus two shard directories as well as the four-shard PR layout.
func TestCIGladecliHistoryRefreshAcceptsTwoAndFourShardDirs(t *testing.T) {
	for _, shards := range []int{2, 4} {
		dir := t.TempDir()
		args := []string{"ci-go-test.sh", "gladecli-history-refresh", filepath.Join(dir, "out.json")}
		for i := 0; i < shards; i++ {
			args = append(args, t.TempDir())
		}
		cmd := exec.Command("bash", args...)
		out, _ := cmd.CombinedOutput()
		if strings.Contains(string(out), "usage: scripts/ci-go-test.sh gladecli-history-refresh") {
			t.Fatalf("%d shard dirs rejected as a usage error:\n%s", shards, out)
		}
	}
	out, _ := exec.Command("bash", "ci-go-test.sh", "gladecli-history-refresh", "only-output").CombinedOutput()
	if !strings.Contains(string(out), "usage: scripts/ci-go-test.sh gladecli-history-refresh") {
		t.Fatalf("missing shard dirs not rejected:\n%s", out)
	}
}
